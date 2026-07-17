// Package polyhaven serves Poly Haven's model library (https://api.polyhaven.com) as an
// assetcore.ModelProvider. Poly Haven's glTF downloads are always multi-file (a main .gltf plus
// referenced textures and a binary buffer), so Fetch assembles them into a single in-memory ZIP rather
// than handing back one URL. The API is keyless — the default httpx User-Agent already satisfies its
// ToS identification requirement — but its Terms of Service restrict the API itself to non-commercial
// use, even though the downloadable assets are CC0. Callers gate registration of this provider on that
// basis; the license this package stamps on every asset reflects only the CC0 asset grant.
package polyhaven

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Poly Haven provider.
const providerName = "polyhaven"

// defaultResolution is the glTF resolution used when a Fetch leaves ModelFetchOpts.Resolution empty.
const defaultResolution = "1k"

// baseURL is the Poly Haven API origin, overridable in tests.
var baseURL = "https://api.polyhaven.com"

// cc0License is the license every Poly Haven model carries: the API's asset library is entirely CC0, so
// this is a package-level constant rather than decoded per hit. Attribution carries the courtesy credit
// the Poly Haven API ToS asks integrators to display next to delivered content, even though CC0 itself
// requires none.
var cc0License = assetcore.License{
	SPDX:                "CC0-1.0",
	Name:                "Creative Commons Zero v1.0 Universal",
	URL:                 "https://creativecommons.org/publicdomain/zero/1.0/",
	RequiresAttribution: false,
	Attribution:         "Model from Poly Haven",
}

// assetMeta is one entry of the object GET /assets?type=models returns, keyed by slug.
type assetMeta struct {
	Name         string   `json:"name"`
	Categories   []string `json:"categories"`
	Tags         []string `json:"tags"`
	ThumbnailURL string   `json:"thumbnail_url"`
}

// filesManifest is the GET /files/{slug} download manifest, narrowed to the gltf branch this provider
// assembles. Gltf maps resolution (e.g. "1k") -> the literal "gltf" bucket -> main filename -> entry.
type filesManifest struct {
	Gltf map[string]map[string]map[string]gltfEntry `json:"gltf"`
}

// gltfEntry describes one main .gltf file: its own download plus every auxiliary file (textures, a
// binary buffer) it references, keyed by the relative path the .gltf expects to find it at.
type gltfEntry struct {
	URL     string             `json:"url"`
	Include map[string]fileRef `json:"include"`
}

// fileRef is one auxiliary file referenced by a gltfEntry's Include map.
type fileRef struct {
	URL string `json:"url"`
}

// asset maps one Poly Haven slug and its assetMeta onto an assetcore.Asset, stamping the composite id
// and the site-wide CC0 license.
func asset(slug string, m assetMeta) assetcore.Asset {
	title := m.Name
	if title == "" {
		title = slug
	}
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, slug),
		Kind:       assetcore.KindModel,
		Title:      title,
		Source:     providerName,
		Tags:       m.Tags,
		License:    cc0License,
		LandingURL: "https://polyhaven.com/a/" + slug,
		PreviewURL: m.ThumbnailURL,
	}
}

// Provider satisfies assetcore.ModelProvider.
var _ assetcore.ModelProvider = (*Provider)(nil)

// Provider serves the Poly Haven model library as an assetcore.ModelProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
}

// New returns a Poly Haven provider using client for HTTP requests, limiter to pace them, and cache to
// avoid re-assembling previously fetched model ZIPs. Poly Haven's API is keyless, so there is no key
// argument.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves 3D models.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindModel }

// Search queries GET /assets?type=models, which returns the entire model catalogue as one object keyed
// by slug, and pages it client-side via an opaque offset cursor (ambientcg-style), since the endpoint
// does not honour a page/limit of its own. opts.Query matches case-insensitively against the slug or
// display name; opts.Sources is checked against each asset's categories. Matching slugs are sorted for a
// stable page order.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("polyhaven: rate limit wait: %w", err)
	}

	var assetsBySlug map[string]assetMeta
	if err := p.client.GetJSON(ctx, baseURL+"/assets?type=models", &assetsBySlug); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("polyhaven: search: %w", err)
	}

	slugs := matchingSlugs(assetsBySlug, opts)

	limit := assetcore.ClampLimit(opts.Limit)
	offset := decodeCursor(opts.Cursor)
	if offset > len(slugs) {
		offset = len(slugs)
	}
	end := offset + limit
	if end > len(slugs) {
		end = len(slugs)
	}
	page := slugs[offset:end]

	assets := make([]assetcore.Asset, 0, len(page))
	for _, slug := range page {
		assets = append(assets, asset(slug, assetsBySlug[slug]))
	}

	result := assetcore.SearchResult{Assets: assets}
	if end < len(slugs) {
		result.NextCursor = strconv.Itoa(end)
	}
	return result, nil
}

// matchingSlugs returns the slugs of assetsBySlug matching opts.Query and opts.Sources, sorted
// ascending for a deterministic page order. An empty Query matches every slug. A slug passes Sources if
// any of its categories does, or if it has no categories at all.
func matchingSlugs(assetsBySlug map[string]assetMeta, opts assetcore.SearchOpts) []string {
	query := strings.ToLower(opts.Query)

	slugs := make([]string, 0, len(assetsBySlug))
	for slug, m := range assetsBySlug {
		if !matchesQuery(query, slug, m.Name) {
			continue
		}
		if !allowsAnyCategory(opts.Sources, m.Categories) {
			continue
		}
		slugs = append(slugs, slug)
	}

	sort.Strings(slugs)
	return slugs
}

// matchesQuery reports whether an empty query matches, or slug or name contains query (already
// lowercased by the caller) as a case-insensitive substring.
func matchesQuery(query, slug, name string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(slug), query) || strings.Contains(strings.ToLower(name), query)
}

// allowsAnyCategory reports whether sources allows at least one of categories, or categories is empty.
func allowsAnyCategory(sources assetcore.Filter, categories []string) bool {
	if len(categories) == 0 {
		return true
	}
	for _, c := range categories {
		if sources.Allows(c) {
			return true
		}
	}
	return false
}

// decodeCursor parses an opaque Search cursor back into an offset, treating "" or an invalid/negative
// value as the first page.
func decodeCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// Fetch assembles a ZIP of the glTF identified by the provider-local slug id, at the resolution selected
// by opts (defaulting to defaultResolution, falling back to the smallest resolution the asset actually
// offers). A cache hit skips both the files-manifest lookup and every file download. An unknown slug, or
// one with no glTF files at any resolution, is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, opts assetcore.ModelFetchOpts) (assetcore.Blob, error) {
	resolution := opts.Resolution
	if resolution == "" {
		resolution = defaultResolution
	}

	zipKey := cache.Key(providerName, id, resolution)
	if data, ok, err := p.cache.Get(zipKey); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polyhaven: cache get %s: %w", zipKey, err)
	} else if ok {
		return blobFor(id, resolution, data), nil
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polyhaven: rate limit wait: %w", err)
	}

	var manifest filesManifest
	filesURL := baseURL + "/files/" + url.PathEscape(id)
	if err := p.client.GetJSON(ctx, filesURL, &manifest); err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return assetcore.Blob{}, fmt.Errorf("polyhaven: asset %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("polyhaven: fetch files manifest: %w", err)
	}

	bucket, ok := gltfBucket(manifest, resolution)
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("polyhaven: asset %q has no gltf files: %w", id, assetcore.ErrNotFound)
	}

	names := make([]string, 0, len(bucket))
	for name := range bucket {
		names = append(names, name)
	}
	sort.Strings(names)
	mainName := names[0]
	entry := bucket[mainName]

	zipData, err := p.assembleZip(ctx, mainName, entry)
	if err != nil {
		return assetcore.Blob{}, err
	}

	if err := p.cache.Put(zipKey, zipData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polyhaven: cache put %s: %w", zipKey, err)
	}

	return blobFor(id, resolution, zipData), nil
}

// gltfBucket returns the main-file bucket (filename -> entry) for resolution, falling back to the
// bucket of the smallest resolution key the manifest actually offers when resolution is absent or
// empty. It reports false when no resolution in the manifest carries a non-empty gltf bucket.
func gltfBucket(m filesManifest, resolution string) (map[string]gltfEntry, bool) {
	if bucket, ok := nonEmptyGltfBucket(m.Gltf[resolution]); ok {
		return bucket, true
	}

	resolutions := make([]string, 0, len(m.Gltf))
	for res := range m.Gltf {
		resolutions = append(resolutions, res)
	}
	sortResolutions(resolutions)

	for _, res := range resolutions {
		if bucket, ok := nonEmptyGltfBucket(m.Gltf[res]); ok {
			return bucket, true
		}
	}
	return nil, false
}

// sortResolutions sorts resolution keys (e.g. "1k", "2k", "4k", "16k") by their leading numeric value
// ascending, so "16k" doesn't precede "2k" as it would under plain lexicographic order. Keys whose
// leading run isn't numeric, or that tie numerically, fall back to lexicographic comparison.
func sortResolutions(resolutions []string) {
	sort.Slice(resolutions, func(i, j int) bool {
		a, aOK := leadingInt(resolutions[i])
		b, bOK := leadingInt(resolutions[j])
		if aOK && bOK && a != b {
			return a < b
		}
		return resolutions[i] < resolutions[j]
	})
}

// leadingInt parses the leading run of ASCII digits in s (e.g. "16k" -> 16), reporting false if s does
// not begin with a digit.
func leadingInt(s string) (int, bool) {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}

// nonEmptyGltfBucket extracts the literal "gltf" bucket from one resolution's entry in
// filesManifest.Gltf, reporting false if it is absent or empty.
func nonEmptyGltfBucket(byBucket map[string]map[string]gltfEntry) (map[string]gltfEntry, bool) {
	bucket, ok := byBucket["gltf"]
	if !ok || len(bucket) == 0 {
		return nil, false
	}
	return bucket, true
}

// assembleZip downloads mainEntry's own file plus every file in its Include map, and packs them into an
// in-memory ZIP with mainName and each Include key used verbatim as the ZIP entry path, so the archive
// preserves the relative layout the .gltf expects.
func (p *Provider) assembleZip(ctx context.Context, mainName string, mainEntry gltfEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := p.addZipFile(ctx, zw, mainName, mainEntry.URL); err != nil {
		return nil, err
	}

	relPaths := make([]string, 0, len(mainEntry.Include))
	for relPath := range mainEntry.Include {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)

	for _, relPath := range relPaths {
		if err := p.addZipFile(ctx, zw, relPath, mainEntry.Include[relPath].URL); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("polyhaven: close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// addZipFile downloads srcURL and writes it into zw at path, rate-limiting the download first.
func (p *Provider) addZipFile(ctx context.Context, zw *zip.Writer, path, srcURL string) error {
	if err := p.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("polyhaven: rate limit wait: %w", err)
	}

	data, err := p.client.GetBytes(ctx, srcURL)
	if err != nil {
		return fmt.Errorf("polyhaven: download %s: %w", srcURL, err)
	}

	w, err := zw.Create(path)
	if err != nil {
		return fmt.Errorf("polyhaven: create zip entry %s: %w", path, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("polyhaven: write zip entry %s: %w", path, err)
	}
	return nil
}

// blobFor builds the assetcore.Blob for an assembled (or cache-rebuilt) model ZIP. On a cache hit, the
// Asset is rebuilt from id alone (title falls back to the slug); Search is the only source of richer
// display metadata.
func blobFor(id, resolution string, content []byte) assetcore.Blob {
	return assetcore.Blob{
		Asset:       asset(id, assetMeta{Name: id}),
		Content:     content,
		Filename:    fmt.Sprintf("%s_%s.zip", id, resolution),
		ContentType: "application/zip",
	}
}

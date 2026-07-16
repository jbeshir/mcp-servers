// Package googlefonts serves the Google Fonts catalogue as woff2 files with optional @font-face CSS,
// implementing assetcore.FontProvider, assetcore.FontFaceRenderer, and assetcore.SourceLister. The
// family/category/license index is embedded at build time; font bytes are fetched from Google's CSS2
// and gstatic endpoints on demand and cached on disk.
package googlefonts

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

//go:embed data/families.json
var familiesFile embed.FS

// ErrNotFound is returned when a requested font family does not exist in the embedded index.
var ErrNotFound = errors.New("font not found")

// providerName is the stable registry key for the Google Fonts provider.
const providerName = "googlefonts"

// woff2ContentType is the MIME type of the downloaded font files.
const woff2ContentType = "font/woff2"

// browserUserAgent is a modern Chrome desktop User-Agent, required because Google's css2 endpoint only
// serves woff2 (rather than woff/ttf) src urls to browser-like clients.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

const (
	defaultWeight = 400
	styleNormal   = "normal"
	styleItalic   = "italic"
)

// css2BaseURL is a package-level var so tests can redirect the css2 endpoint to an httptest server.
var css2BaseURL = "https://fonts.googleapis.com/css2"

// woff2URLPattern extracts the first woff2 src url from a css2 response body, e.g.
// `src: url(https://fonts.gstatic.com/s/roboto/v30/....woff2) format('woff2');`.
var woff2URLPattern = regexp.MustCompile(`url\((https?://[^)]+\.woff2)\)`)

// familyEntry is one row of the embedded families index.
type familyEntry struct {
	Family   string `json:"family"`
	Category string `json:"category"`
	License  string `json:"license"`
}

// licenseNames maps the SPDX identifiers carried by the embedded index to their human-readable names.
var licenseNames = map[string]string{
	"OFL-1.1":    "SIL Open Font License 1.1",
	"Apache-2.0": "Apache License 2.0",
	"UFL-1.0":    "Ubuntu Font License 1.0",
}

// families holds the parsed embedded index, loaded once by loadFamilies.
var families []familyEntry

// familyBySlug maps each family's slug to its index entry, loaded once by loadFamilies.
var familyBySlug map[string]familyEntry

var (
	loadFamiliesOnce sync.Once
	loadErr          error
)

// loadFamilies parses the embedded families.json, once, capturing the first read or parse failure in
// loadErr so New can report a corrupt embed distinctly from a legitimately empty index.
func loadFamilies() error {
	loadFamiliesOnce.Do(func() {
		data, err := familiesFile.ReadFile("data/families.json")
		if err != nil {
			loadErr = fmt.Errorf("read families.json: %w", err)
			return
		}

		var entries []familyEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			loadErr = fmt.Errorf("parse families.json: %w", err)
			return
		}

		byslug := make(map[string]familyEntry, len(entries))
		for _, e := range entries {
			byslug[slugify(e.Family)] = e
		}

		families = entries
		familyBySlug = byslug
	})

	return loadErr
}

// slugify normalizes a font family display name or slug into its canonical slug form.
func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

// licenseFor builds the assetcore.License for a family's SPDX identifier. Google Fonts' OFL/Apache/UFL
// licenses do not require end-user attribution for the delivered woff2 (matching embeddedfonts).
func licenseFor(spdx string) assetcore.License {
	return assetcore.License{SPDX: spdx, Name: licenseNames[spdx]}
}

// Provider satisfies assetcore.FontProvider and the optional FontFaceRenderer/SourceLister capabilities.
var (
	_ assetcore.FontProvider     = (*Provider)(nil)
	_ assetcore.FontFaceRenderer = (*Provider)(nil)
	_ assetcore.SourceLister     = (*Provider)(nil)
)

// Provider serves the Google Fonts catalogue as an assetcore.FontProvider, downloading and caching
// woff2 bytes on demand from Google's css2/gstatic endpoints.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
}

// New returns a Google Fonts provider. It panics if the embedded families index fails to read or
// parse, since that indicates a corrupt embed rather than a legitimately empty index.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache) *Provider {
	if err := loadFamilies(); err != nil {
		panic(fmt.Errorf("googlefonts: load families: %w", err))
	}

	return &Provider{client: client, limiter: limiter, cache: cache}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves fonts.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindFont }

// Search finds font families matching opts.Query among those allowed by opts.Sources and maps each hit
// onto an assetcore.Asset, carrying the family's category as display Meta. The embedded catalogue fits
// in a single page, so NextCursor is always "". No network is involved.
func (p *Provider) Search(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	q := strings.ToLower(opts.Query)
	limit := assetcore.ClampLimit(opts.Limit)

	var assets []assetcore.Asset
	for _, e := range families {
		if !opts.Sources.Allows(e.Family) {
			continue
		}

		slug := slugify(e.Family)
		if !strings.Contains(strings.ToLower(e.Family), q) &&
			!strings.Contains(slug, q) &&
			!strings.Contains(strings.ToLower(e.Category), q) {
			continue
		}

		assets = append(assets, p.asset(e))
		if len(assets) >= limit {
			break
		}
	}

	return assetcore.SearchResult{Assets: assets}, nil
}

// asset builds the search-hit assetcore.Asset for a family, carrying its category as display Meta.
func (p *Provider) asset(e familyEntry) assetcore.Asset {
	return assetcore.Asset{
		Source:  e.Family,
		ID:      assetcore.AssetID(providerName, slugify(e.Family)),
		Kind:    assetcore.KindFont,
		Title:   e.Family,
		License: licenseFor(e.License),
		Meta:    map[string]string{assetcore.MetaCategory: e.Category},
	}
}

// Fetch returns the woff2 bytes for the family identified by the provider-local slug id, at the weight
// and style in opts (defaulting to 400/normal), downloading and caching them on first use. The Blob's
// Filename encodes the slug/weight/style so RenderFontFace can recover them later. An unknown family is
// reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, opts assetcore.FontFetchOpts) (assetcore.Blob, error) {
	entry, ok := familyBySlug[id]
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("font family %q: %w", id, errors.Join(assetcore.ErrNotFound, ErrNotFound))
	}

	weight := opts.Weight
	if weight == 0 {
		weight = defaultWeight
	}

	style := opts.Style
	if style == "" {
		style = styleNormal
	}

	filename := fontFilename(id, weight, style)
	cacheKey := cache.Key(providerName, id, strconv.Itoa(weight), style)

	data, ok, err := p.cache.Get(cacheKey)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("googlefonts: cache get %s: %w", cacheKey, err)
	}

	if !ok {
		data, err = p.download(ctx, entry.Family, weight, style)
		if err != nil {
			return assetcore.Blob{}, err
		}

		if err := p.cache.Put(cacheKey, data); err != nil {
			return assetcore.Blob{}, fmt.Errorf("googlefonts: cache put %s: %w", cacheKey, err)
		}
	}

	return assetcore.Blob{
		Asset: assetcore.Asset{
			Source:  entry.Family,
			ID:      assetcore.AssetID(providerName, id),
			Kind:    assetcore.KindFont,
			Title:   entry.Family,
			License: licenseFor(entry.License),
		},
		Content:     data,
		Filename:    filename,
		ContentType: woff2ContentType,
	}, nil
}

// download fetches the css2 stylesheet for family/weight/style, extracts the first gstatic woff2 src
// url, and downloads its bytes, applying the rate limiter before each outbound request.
func (p *Provider) download(ctx context.Context, family string, weight int, style string) ([]byte, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("googlefonts: rate limit wait: %w", err)
	}

	cssURL := css2URL(family, weight, style)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("googlefonts: build css2 request: %w", err)
	}
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("googlefonts: fetch css2 %s: %w", cssURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := httpx.CheckStatus(resp, cssURL); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("googlefonts: read css2 %s: %w", cssURL, err)
	}

	woff2URL, err := extractWoff2URL(body)
	if err != nil {
		return nil, fmt.Errorf("googlefonts: family %q weight %d style %s: %w", family, weight, style, err)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("googlefonts: rate limit wait: %w", err)
	}

	data, err := p.client.GetBytes(ctx, woff2URL)
	if err != nil {
		return nil, fmt.Errorf("googlefonts: download woff2 %s: %w", woff2URL, err)
	}

	return data, nil
}

// errNoWoff2Src is returned when a css2 response contains no gstatic woff2 src url.
var errNoWoff2Src = errors.New("no woff2 src found in css2 response")

// extractWoff2URL scans a css2 response body for the first `url(<...>.woff2)` reference.
func extractWoff2URL(cssBody []byte) (string, error) {
	m := woff2URLPattern.FindSubmatch(cssBody)
	if m == nil {
		return "", errNoWoff2Src
	}

	return string(m[1]), nil
}

// css2URL builds the Google Fonts css2 request URL for family/weight/style, encoding spaces in the
// family name as '+' (not percent-escaped, per the css2 API's expected query form).
func css2URL(family string, weight int, style string) string {
	plusFamily := strings.ReplaceAll(family, " ", "+")

	ital := 0
	if style == styleItalic {
		ital = 1
	}

	return fmt.Sprintf("%s?family=%s:ital,wght@%d,%d", css2BaseURL, plusFamily, ital, weight)
}

// fontFilename builds the internal font filename encoding slug/weight/style, so RenderFontFace can
// recover them from a Blob alone.
func fontFilename(slug string, weight int, style string) string {
	return fmt.Sprintf("%s-%d-%s.woff2", slug, weight, style)
}

// parseFontFilename recovers the weight and style encoded in a filename built by fontFilename
// (<slug>-<weight>-<style>.woff2).
func parseFontFilename(filename string) (weight int, style string) {
	base := strings.TrimSuffix(filename, ".woff2")

	idx := strings.LastIndex(base, "-")
	if idx < 0 {
		return 0, ""
	}

	style = base[idx+1:]
	rest := base[:idx]

	idx = strings.LastIndex(rest, "-")
	if idx < 0 {
		return 0, ""
	}

	w, err := strconv.Atoi(rest[idx+1:])
	if err != nil {
		return 0, ""
	}

	return w, style
}

// RenderFontFace renders an @font-face CSS snippet for a font Blob produced by Fetch, recovering the
// weight and style from the Blob's internal filename. It satisfies assetcore.FontFaceRenderer.
func (p *Provider) RenderFontFace(familyDisplay string, b assetcore.Blob) string {
	weight, style := parseFontFilename(b.Filename)

	return fmt.Sprintf(
		"@font-face {\n  font-family: %q;\n  font-style: %s;\n  font-weight: %d;\n  src: url(%q) format(\"woff2\");\n}\n",
		familyDisplay, style, weight, b.Filename,
	)
}

// Sources reports one assetcore.Source per indexed family, with its license and category as display
// Meta. Count is -1 since weight availability is not modelled by the embedded index.
func (p *Provider) Sources() []assetcore.Source {
	out := make([]assetcore.Source, 0, len(families))
	for _, e := range families {
		out = append(out, assetcore.Source{
			Name:    e.Family,
			License: licenseFor(e.License),
			Count:   -1,
			Meta:    map[string]string{assetcore.MetaCategory: e.Category},
		})
	}

	return out
}

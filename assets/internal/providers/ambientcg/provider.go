// Package ambientcg serves ambientCG's PBR material library as an assetcore.TextureProvider, backed by
// the public ambientCG full_json API. Every asset on the site is licensed CC0-1.0; the API carries no
// per-asset license field, so that license is a package-level constant rather than decoded per hit.
package ambientcg

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the ambientCG provider.
const providerName = "ambientcg"

// apiBaseURL is the ambientCG API origin, overridable in tests.
var apiBaseURL = "https://ambientcg.com"

// apiPath is the full_json search/lookup endpoint, relative to apiBaseURL.
const apiPath = "/api/v2/full_json"

// defaultResolution and defaultFormat select the download attribute used when TextureFetchOpts leaves
// either field empty.
const (
	defaultResolution = "1K"
	defaultFormat     = "JPG"
)

// cc0License is the site-wide license for every ambientCG asset; the API carries no per-asset license
// field, so this is stamped onto every Asset and Blob this provider produces.
var cc0License = assetcore.License{
	SPDX:                "CC0-1.0",
	Name:                "Creative Commons Zero v1.0 Universal",
	URL:                 "https://creativecommons.org/publicdomain/zero/1.0/",
	RequiresAttribution: false,
}

// searchEnvelope is the top-level shape of a full_json response.
type searchEnvelope struct {
	NumberOfResults int          `json:"numberOfResults"`
	FoundAssets     []foundAsset `json:"foundAssets"`
}

// foundAsset is one material entry in a full_json response.
type foundAsset struct {
	AssetID         string                    `json:"assetId"`
	DisplayName     string                    `json:"displayName"`
	DisplayCategory string                    `json:"displayCategory"`
	Tags            []string                  `json:"tags"`
	ShortLink       string                    `json:"shortLink"`
	DownloadFolders map[string]downloadFolder `json:"downloadFolders"`
}

// downloadFolder is one entry in a foundAsset's downloadFolders map; its key is opaque and not relied
// upon, so only the nested filetype categories are decoded.
type downloadFolder struct {
	DownloadFiletypeCategories map[string]filetypeCategory `json:"downloadFiletypeCategories"`
}

// filetypeCategory groups the individual downloads offered for one filetype (e.g. "zip").
type filetypeCategory struct {
	Downloads []downloadEntry `json:"downloads"`
}

// downloadEntry is a single downloadable archive. Attribute is a "<Resolution>-<Format>" string, e.g.
// "1K-JPG", and is the sole field used to select among the downloads offered for an asset.
type downloadEntry struct {
	DownloadLink string `json:"downloadLink"`
	FileName     string `json:"fileName"`
	Attribute    string `json:"attribute"`
	Filetype     string `json:"filetype"`
}

// Provider satisfies assetcore.TextureProvider.
var _ assetcore.TextureProvider = (*Provider)(nil)

// Provider serves the ambientCG material library as an assetcore.TextureProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
}

// New returns an ambientCG provider using client for HTTP requests, limiter to pace them, and cache to
// avoid re-downloading previously fetched archives.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves textures.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindTexture }

// Search queries the ambientCG full_json endpoint for materials matching opts.Query, paging via an
// opaque offset cursor and honouring opts.Sources against each hit's display category.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("ambientcg: rate limit wait: %w", err)
	}

	limit := assetcore.ClampLimit(opts.Limit)
	offset := decodeCursor(opts.Cursor)

	q := url.Values{}
	q.Set("type", "Material")
	q.Set("q", opts.Query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	q.Set("include", "downloadData")

	var env searchEnvelope
	if err := p.client.GetJSON(ctx, apiBaseURL+apiPath+"?"+q.Encode(), &env); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("ambientcg: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(env.FoundAssets))
	for _, fa := range env.FoundAssets {
		source := fa.DisplayCategory
		if source == "" {
			source = providerName
		}
		if !opts.Sources.Allows(source) {
			continue
		}
		assets = append(assets, asset(fa, source))
	}

	result := assetcore.SearchResult{Assets: assets}
	if n := len(env.FoundAssets); n > 0 && offset+n < env.NumberOfResults {
		result.NextCursor = strconv.Itoa(offset + n)
	}
	return result, nil
}

// Fetch downloads the material archive for the asset identified by the provider-local id (the
// ambientCG assetId), at the resolution and format selected by opts (defaulting to 1K/JPG). A cache hit
// skips both the metadata lookup and the archive download. An unknown asset id, or one with no download
// matching the requested resolution/format, is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, opts assetcore.TextureFetchOpts) (assetcore.Blob, error) {
	resolution := opts.Resolution
	if resolution == "" {
		resolution = defaultResolution
	}
	format := opts.Format
	if format == "" {
		format = defaultFormat
	}

	cacheKey := cache.Key(providerName, id, resolution, format)
	if data, ok, err := p.cache.Get(cacheKey); err != nil {
		return assetcore.Blob{}, fmt.Errorf("ambientcg: cache get %s: %w", cacheKey, err)
	} else if ok {
		return blobFor(id, resolution, format, data), nil
	}

	downloadLink, err := p.resolveDownloadLink(ctx, id, resolution, format)
	if err != nil {
		return assetcore.Blob{}, err
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("ambientcg: rate limit wait: %w", err)
	}

	content, err := p.client.GetBytes(ctx, downloadLink)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("ambientcg: download %s: %w", downloadLink, err)
	}

	if err := p.cache.Put(cacheKey, content); err != nil {
		return assetcore.Blob{}, fmt.Errorf("ambientcg: cache put %s: %w", cacheKey, err)
	}

	return blobFor(id, resolution, format, content), nil
}

// resolveDownloadLink looks up id via the full_json id= parameter (an exact single-asset lookup; the q=
// search does not resolve an assetId) and returns the downloadLink whose attribute matches
// "<resolution>-<format>". An unknown asset id, or one with no matching download, is reported as
// assetcore.ErrNotFound.
func (p *Provider) resolveDownloadLink(ctx context.Context, id, resolution, format string) (string, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("ambientcg: rate limit wait: %w", err)
	}

	q := url.Values{}
	q.Set("id", id)
	q.Set("include", "downloadData")

	var env searchEnvelope
	if err := p.client.GetJSON(ctx, apiBaseURL+apiPath+"?"+q.Encode(), &env); err != nil {
		return "", fmt.Errorf("ambientcg: lookup %s: %w", id, err)
	}

	fa, ok := findFoundAsset(env.FoundAssets, id)
	if !ok {
		return "", fmt.Errorf("ambientcg: asset %q: %w", id, assetcore.ErrNotFound)
	}

	link, ok := findDownloadLink(fa.DownloadFolders, resolution+"-"+format)
	if !ok {
		return "", fmt.Errorf("ambientcg: asset %q has no %s-%s download: %w", id, resolution, format, assetcore.ErrNotFound)
	}
	return link, nil
}

// findFoundAsset returns the foundAsset whose AssetID matches id.
func findFoundAsset(assets []foundAsset, id string) (foundAsset, bool) {
	for _, fa := range assets {
		if fa.AssetID == id {
			return fa, true
		}
	}
	return foundAsset{}, false
}

// findDownloadLink walks folders (downloadFolders -> downloadFiletypeCategories -> downloads),
// returning the downloadLink of the first entry whose attribute equals attr. Folder and filetype
// category keys are not relied upon, since ambientCG does not document them as stable.
func findDownloadLink(folders map[string]downloadFolder, attr string) (string, bool) {
	for _, folder := range folders {
		for _, cat := range folder.DownloadFiletypeCategories {
			for _, d := range cat.Downloads {
				if d.Attribute == attr {
					return d.DownloadLink, true
				}
			}
		}
	}
	return "", false
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

// asset maps one full_json foundAsset onto an assetcore.Asset, stamping the composite id and the
// site-wide CC0 license.
func asset(fa foundAsset, source string) assetcore.Asset {
	title := fa.DisplayName
	if title == "" {
		title = fa.AssetID
	}
	return assetcore.Asset{
		Source:     source,
		ID:         assetcore.AssetID(providerName, fa.AssetID),
		Kind:       assetcore.KindTexture,
		Title:      title,
		Tags:       fa.Tags,
		License:    cc0License,
		LandingURL: fa.ShortLink,
	}
}

// blobFor builds the assetcore.Blob for a fetched material archive.
func blobFor(id, resolution, format string, content []byte) assetcore.Blob {
	return assetcore.Blob{
		Asset: assetcore.Asset{
			ID:      assetcore.AssetID(providerName, id),
			Kind:    assetcore.KindTexture,
			Title:   id,
			License: cc0License,
		},
		Content:     content,
		Filename:    fmt.Sprintf("%s_%s-%s.zip", id, resolution, format),
		ContentType: "application/zip",
	}
}

// Package openverse serves the Openverse API (https://api.openverse.org) as an assetcore.PhotoProvider,
// searching its catalogue of openly licensed images and fetching the underlying bytes on demand.
package openverse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Openverse provider.
const providerName = "openverse"

// baseURL is the Openverse API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://api.openverse.org"

// defaultPage is the page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// searchResult is the Openverse search envelope returned by GET /v1/images/.
type searchResult struct {
	ResultCount int           `json:"result_count"`
	PageCount   int           `json:"page_count"`
	Page        int           `json:"page"`
	PageSize    int           `json:"page_size"`
	Results     []imageResult `json:"results"`
}

// imageResult is a single Openverse image record, shared by the search envelope and the detail
// endpoint (GET /v1/images/<uuid>/).
type imageResult struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	ForeignLandingURL string `json:"foreign_landing_url"`
	URL               string `json:"url"`
	Thumbnail         string `json:"thumbnail"`
	Creator           string `json:"creator"`
	License           string `json:"license"`
	LicenseVersion    string `json:"license_version"`
	LicenseURL        string `json:"license_url"`
	Attribution       string `json:"attribution"`
	Source            string `json:"source"`
}

// extToContentType maps a lowercase file extension (without the leading dot) to its MIME type.
var extToContentType = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
	"svg":  "image/svg+xml",
}

// defaultExt and defaultContentType are used when the image URL's extension is unrecognized or absent.
const (
	defaultExt         = ".img"
	defaultContentType = "application/octet-stream"
)

// Openverse license codes with special handling: both denote public-domain dedications that carry no
// SPDX-style CC-BY family identifier and require no attribution.
const (
	codeCC0 = "cc0"
	codePDM = "pdm"
)

// spdxFor maps an Openverse license code and version to its SPDX identifier. "pdm" (public domain
// mark) has no SPDX identifier, so it maps to "".
func spdxFor(code, version string) string {
	switch code {
	case codeCC0:
		return "CC0-1.0"
	case codePDM:
		return ""
	default:
		return "CC-" + strings.ToUpper(code) + "-" + version
	}
}

// humanName maps an Openverse license code and version to a human-readable label.
func humanName(code, version string) string {
	switch code {
	case codeCC0:
		return "CC0 " + version
	case codePDM:
		return "Public Domain Mark " + version
	default:
		return "CC " + strings.ToUpper(strings.ReplaceAll(code, "-", " ")) + " " + version
	}
}

// requiresAttribution reports whether an Openverse license code requires attribution. Public domain
// works ("cc0", "pdm") do not.
func requiresAttribution(code string) bool {
	return code != codeCC0 && code != codePDM
}

// contentTypeForURL derives the MIME type and file extension (including the leading dot) from imgURL's
// path extension, falling back to defaultContentType/defaultExt when unrecognized.
func contentTypeForURL(imgURL string) (contentType, ext string) {
	parsed, err := url.Parse(imgURL)
	if err != nil {
		return defaultContentType, defaultExt
	}
	ext = strings.ToLower(path.Ext(parsed.Path))
	contentType, ok := extToContentType[strings.TrimPrefix(ext, ".")]
	if !ok {
		return defaultContentType, defaultExt
	}
	return contentType, ext
}

// license builds the assetcore.License for an Openverse image result.
func license(r imageResult) assetcore.License {
	return assetcore.License{
		SPDX:                spdxFor(r.License, r.LicenseVersion),
		Name:                humanName(r.License, r.LicenseVersion),
		URL:                 r.LicenseURL,
		Attribution:         r.Attribution,
		RequiresAttribution: requiresAttribution(r.License),
	}
}

// asset builds the assetcore.Asset for an Openverse image result.
func asset(r imageResult) assetcore.Asset {
	return assetcore.Asset{
		Source:     r.Source,
		ID:         assetcore.AssetID(providerName, r.ID),
		Kind:       assetcore.KindPhoto,
		Title:      r.Title,
		License:    license(r),
		LandingURL: r.ForeignLandingURL,
		PreviewURL: r.Thumbnail,
	}
}

// Provider satisfies assetcore.PhotoProvider.
var _ assetcore.PhotoProvider = (*Provider)(nil)

// Provider serves the Openverse API as an assetcore.PhotoProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
}

// New returns an Openverse provider using client for HTTP requests, limiter to pace outbound requests,
// and cache to avoid re-downloading previously fetched image bytes.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves photos.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindPhoto }

// Search queries the Openverse image search endpoint for opts.Query, honouring opts.Cursor as a page
// number (default 1) and opts.Limit as the page size.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("openverse: rate limit wait: %w", err)
	}

	page := defaultPage
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		page = n
	}

	q := url.Values{
		"q":         {opts.Query},
		"page":      {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(assetcore.ClampLimit(opts.Limit))},
	}
	reqURL := baseURL + "/v1/images/?" + q.Encode()

	var result searchResult
	if err := p.client.GetJSON(ctx, reqURL, &result); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("openverse: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(result.Results))
	for _, r := range result.Results {
		assets = append(assets, asset(r))
	}

	var nextCursor string
	if result.Page < result.PageCount {
		nextCursor = strconv.Itoa(result.Page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// imageCacheKey and metaCacheKey return the on-disk cache keys for id's image bytes and detail
// metadata respectively, namespaced by provider so the two never collide.
func imageCacheKey(id string) string { return providerName + "\x00img\x00" + id }
func metaCacheKey(id string) string  { return providerName + "\x00meta\x00" + id }

// Fetch returns the image identified by the provider-local uuid id, checking the on-disk cache for
// both the image bytes and the detail metadata (license, title, landing/preview URLs) before making any
// network call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP
// requests. On a miss, it performs the detail GET, downloads the image bytes, and caches both before
// returning. A 404 detail response is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, _ assetcore.PhotoFetchOpts) (assetcore.Blob, error) {
	imgKey := imageCacheKey(id)
	metaKey := metaCacheKey(id)

	blob, hit, err := p.cachedBlob(id, imgKey, metaKey)
	if err != nil {
		return assetcore.Blob{}, err
	}
	if hit {
		return blob, nil
	}

	return p.fetchAndCache(ctx, id, imgKey, metaKey)
}

// cachedBlob rebuilds the Blob for id from the on-disk cache when both its image bytes and detail
// metadata are present, returning hit=false with a zero Blob when either entry is missing.
func (p *Provider) cachedBlob(id, imgKey, metaKey string) (assetcore.Blob, bool, error) {
	imgData, imgHit, err := p.cache.Get(imgKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("openverse: cache get %s: %w", imgKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("openverse: cache get %s: %w", metaKey, err)
	}
	if !imgHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var r imageResult
	if err := json.Unmarshal(metaData, &r); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("openverse: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, r, imgData), true, nil
}

// fetchAndCache performs the detail lookup and image download for id, caching both the metadata and
// the image bytes before returning the Blob. A 404 detail response is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(ctx context.Context, id, imgKey, metaKey string) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: rate limit wait: %w", err)
	}

	detailURL := baseURL + "/v1/images/" + url.PathEscape(id) + "/"
	var r imageResult
	if err := p.client.GetJSON(ctx, detailURL, &r); err != nil {
		var statusErr *httpx.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			return assetcore.Blob{}, fmt.Errorf("openverse: image %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("openverse: fetch detail: %w", err)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: rate limit wait: %w", err)
	}
	imgData, err := p.client.GetBytes(ctx, r.URL)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: fetch image: %w", err)
	}

	metaData, err := json.Marshal(r)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(imgKey, imgData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("openverse: cache put %s: %w", imgKey, err)
	}

	return blobFor(id, r, imgData), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) image, deriving the content type
// and filename extension from r's source URL.
func blobFor(id string, r imageResult, content []byte) assetcore.Blob {
	contentType, ext := contentTypeForURL(r.URL)
	return assetcore.Blob{
		Asset:       asset(r),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

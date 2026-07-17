// Package pexels serves the Pexels API (https://api.pexels.com) as an assetcore.PhotoProvider,
// searching Pexels' photo catalogue and fetching the underlying bytes on demand. Every request against
// api.pexels.com is authenticated with the caller-supplied API key; the package never reads the
// environment itself.
package pexels

import (
	"context"
	"encoding/json"
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

// providerName is the stable registry key for the Pexels provider.
const providerName = "pexels"

// baseURL is the Pexels API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://api.pexels.com"

// defaultPage is the page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// maxPerPage is the largest per_page value Pexels' search endpoint accepts.
const maxPerPage = 80

// searchResult is the Pexels search envelope returned by GET /v1/search.
type searchResult struct {
	TotalResults int     `json:"total_results"`
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
	Photos       []photo `json:"photos"`
	NextPage     string  `json:"next_page"`
}

// photoSrc is the set of sized image URLs carried by a photo record.
type photoSrc struct {
	Original  string `json:"original"`
	Large2X   string `json:"large2x"`
	Large     string `json:"large"`
	Medium    string `json:"medium"`
	Small     string `json:"small"`
	Portrait  string `json:"portrait"`
	Landscape string `json:"landscape"`
	Tiny      string `json:"tiny"`
}

// photo is a single Pexels photo record, shared by the search envelope and the detail endpoint
// (GET /v1/photos/{id}).
type photo struct {
	ID           int      `json:"id"`
	URL          string   `json:"url"`
	Photographer string   `json:"photographer"`
	Alt          string   `json:"alt"`
	Src          photoSrc `json:"src"`
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

// title picks the display title for p: its alt text if non-empty, otherwise a photographer credit.
func title(p photo) string {
	if p.Alt != "" {
		return p.Alt
	}
	return "Photo by " + p.Photographer
}

// license builds the assetcore.License for a photo credited to photographer. Pexels has no SPDX-style
// license identifier of its own; every photo requires attribution to both photographer and Pexels.
func license(photographer string) assetcore.License {
	return assetcore.License{
		SPDX:                "",
		Name:                "Pexels License",
		URL:                 "https://www.pexels.com/license/",
		Attribution:         "Photo by " + photographer + " on Pexels",
		RequiresAttribution: true,
	}
}

// asset builds the assetcore.Asset for a Pexels photo record.
func asset(p photo) assetcore.Asset {
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, strconv.Itoa(p.ID)),
		Kind:       assetcore.KindPhoto,
		Title:      title(p),
		License:    license(p.Photographer),
		LandingURL: p.URL,
		PreviewURL: p.Src.Tiny,
	}
}

// Provider satisfies assetcore.PhotoProvider.
var _ assetcore.PhotoProvider = (*Provider)(nil)

// Provider serves the Pexels API as an assetcore.PhotoProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
	apiKey  string
}

// New returns a Pexels provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched image bytes, and apiKey to authenticate every
// request against api.pexels.com.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, apiKey string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, apiKey: apiKey}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves photos.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindPhoto }

// authHeader builds the header set required by every api.pexels.com request: the raw API key with no
// scheme prefix.
func (p *Provider) authHeader() http.Header {
	return http.Header{"Authorization": {p.apiKey}}
}

// Search queries the Pexels photo search endpoint for opts.Query, honouring opts.Cursor as a page
// number (default 1) and opts.Limit as the page size, capped at Pexels' per_page maximum of 80.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("pexels: rate limit wait: %w", err)
	}

	page := defaultPage
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		page = n
	}

	perPage := assetcore.ClampLimit(opts.Limit)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	q := url.Values{
		"query":    {opts.Query},
		"page":     {strconv.Itoa(page)},
		"per_page": {strconv.Itoa(perPage)},
	}
	reqURL := baseURL + "/v1/search?" + q.Encode()

	var result searchResult
	if err := p.client.GetJSONHeaders(ctx, reqURL, p.authHeader(), &result); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("pexels: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(result.Photos))
	for _, ph := range result.Photos {
		assets = append(assets, asset(ph))
	}

	var nextCursor string
	if result.NextPage != "" {
		nextCursor = strconv.Itoa(result.Page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// imageCacheKey and metaCacheKey return the on-disk cache keys for id's image bytes and detail metadata
// respectively, namespaced by provider so the two never collide.
func imageCacheKey(id string) string { return cache.Key(providerName, "img", id) }
func metaCacheKey(id string) string  { return cache.Key(providerName, "meta", id) }

// Fetch returns the image identified by the provider-local id, checking the on-disk cache for both the
// image bytes and the detail metadata (license, title, landing/preview URLs) before making any network
// call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP requests. On a
// miss, it performs the detail GET, downloads the image bytes, and caches both before returning. A 404
// detail response is reported as assetcore.ErrNotFound.
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
		return assetcore.Blob{}, false, fmt.Errorf("pexels: cache get %s: %w", imgKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("pexels: cache get %s: %w", metaKey, err)
	}
	if !imgHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var ph photo
	if err := json.Unmarshal(metaData, &ph); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("pexels: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, ph, imgData), true, nil
}

// fetchAndCache performs the detail lookup and image download for id, caching both the metadata and
// the image bytes before returning the Blob. A 404 detail response is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(ctx context.Context, id, imgKey, metaKey string) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: rate limit wait: %w", err)
	}

	detailURL := baseURL + "/v1/photos/" + url.PathEscape(id)
	var ph photo
	if err := p.client.GetJSONHeaders(ctx, detailURL, p.authHeader(), &ph); err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return assetcore.Blob{}, fmt.Errorf("pexels: photo %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("pexels: fetch detail: %w", err)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: rate limit wait: %w", err)
	}
	imgData, err := p.client.GetBytesHeaders(ctx, ph.Src.Original, p.authHeader())
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: fetch image: %w", err)
	}

	metaData, err := json.Marshal(ph)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(imgKey, imgData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pexels: cache put %s: %w", imgKey, err)
	}

	return blobFor(id, ph, imgData), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) image, deriving the content type
// and filename extension from p's full-resolution image URL.
func blobFor(id string, p photo, content []byte) assetcore.Blob {
	contentType, ext := contentTypeForURL(p.Src.Original)
	return assetcore.Blob{
		Asset:       asset(p),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

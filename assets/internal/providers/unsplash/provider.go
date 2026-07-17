// Package unsplash serves the Unsplash API (https://api.unsplash.com) as an assetcore.PhotoProvider,
// searching Unsplash's photo catalogue and fetching the underlying bytes on demand. Every request
// against api.unsplash.com is authenticated with the caller-supplied access key; the package never reads
// the environment itself.
package unsplash

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

// providerName is the stable registry key for the Unsplash provider.
const providerName = "unsplash"

// baseURL is the Unsplash API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://api.unsplash.com"

// defaultPage is the page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// maxPerPage is the largest per_page value Unsplash's search endpoint accepts.
const maxPerPage = 30

// searchResult is the Unsplash search envelope returned by GET /search/photos.
type searchResult struct {
	Total      int     `json:"total"`
	TotalPages int     `json:"total_pages"`
	Results    []photo `json:"results"`
}

// photoURLs is the set of sized image URLs carried by a photo record.
type photoURLs struct {
	Full  string `json:"full"`
	Thumb string `json:"thumb"`
}

// photoLinks is the set of related links carried by a photo record.
type photoLinks struct {
	HTML             string `json:"html"`
	DownloadLocation string `json:"download_location"`
}

// photoUser identifies a photo's creator.
type photoUser struct {
	Name string `json:"name"`
}

// photo is a single Unsplash photo record, shared by the search envelope and the detail endpoint
// (GET /photos/{id}).
type photo struct {
	ID             string     `json:"id"`
	Description    *string    `json:"description"`
	AltDescription *string    `json:"alt_description"`
	Urls           photoURLs  `json:"urls"`
	Links          photoLinks `json:"links"`
	User           photoUser  `json:"user"`
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

// title picks the display title for p: the first non-empty of its description, its alt description, or
// its id.
func title(p photo) string {
	if p.Description != nil && *p.Description != "" {
		return *p.Description
	}
	if p.AltDescription != nil && *p.AltDescription != "" {
		return *p.AltDescription
	}
	return p.ID
}

// license builds the assetcore.License for a photo credited to creatorName. Unsplash has no SPDX-style
// license identifier of its own; every photo requires attribution to both photographer and Unsplash.
func license(creatorName string) assetcore.License {
	return assetcore.License{
		SPDX:                "",
		Name:                "Unsplash License",
		URL:                 "https://unsplash.com/license",
		Attribution:         "Photo by " + creatorName + " on Unsplash",
		RequiresAttribution: true,
	}
}

// asset builds the assetcore.Asset for an Unsplash photo record.
func asset(p photo) assetcore.Asset {
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, p.ID),
		Kind:       assetcore.KindPhoto,
		Title:      title(p),
		Source:     p.User.Name,
		License:    license(p.User.Name),
		LandingURL: p.Links.HTML,
		PreviewURL: p.Urls.Thumb,
	}
}

// Provider satisfies assetcore.PhotoProvider.
var _ assetcore.PhotoProvider = (*Provider)(nil)

// Provider serves the Unsplash API as an assetcore.PhotoProvider.
type Provider struct {
	client    *httpx.Client
	limiter   *ratelimit.Limiter
	cache     *cache.Cache
	accessKey string
}

// New returns an Unsplash provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched image bytes, and accessKey to authenticate every
// request against api.unsplash.com.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, accessKey string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, accessKey: accessKey}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves photos.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindPhoto }

// authHeader builds the header set required by every api.unsplash.com request: the Client-ID
// authorization scheme and the API version pin.
func (p *Provider) authHeader() http.Header {
	return http.Header{
		"Authorization":  {"Client-ID " + p.accessKey},
		"Accept-Version": {"v1"},
	}
}

// Search queries the Unsplash photo search endpoint for opts.Query, honouring opts.Cursor as a page
// number (default 1) and opts.Limit as the page size, capped at Unsplash's per_page maximum of 30.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("unsplash: rate limit wait: %w", err)
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
	reqURL := baseURL + "/search/photos?" + q.Encode()

	var result searchResult
	if err := p.client.GetJSONHeaders(ctx, reqURL, p.authHeader(), &result); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("unsplash: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(result.Results))
	for _, ph := range result.Results {
		assets = append(assets, asset(ph))
	}

	var nextCursor string
	if page < result.TotalPages {
		nextCursor = strconv.Itoa(page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// imageCacheKey and metaCacheKey return the on-disk cache keys for id's image bytes and detail metadata
// respectively, namespaced by provider so the two never collide.
func imageCacheKey(id string) string { return cache.Key(providerName, "img", id) }
func metaCacheKey(id string) string  { return cache.Key(providerName, "meta", id) }

// Fetch returns the image identified by the provider-local id, checking the on-disk cache for both the
// image bytes and the detail metadata (license, title, landing/preview URLs) before making any network
// call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP requests and,
// critically, no re-trigger of Unsplash's download-tracking endpoint. On a miss, it performs the detail
// GET, triggers the download-tracking endpoint required by Unsplash's API guidelines, downloads the
// image bytes, and caches both before returning. A 404 detail response is reported as
// assetcore.ErrNotFound.
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
		return assetcore.Blob{}, false, fmt.Errorf("unsplash: cache get %s: %w", imgKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("unsplash: cache get %s: %w", metaKey, err)
	}
	if !imgHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var ph photo
	if err := json.Unmarshal(metaData, &ph); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("unsplash: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, ph, imgData), true, nil
}

// fetchAndCache performs the detail lookup, the mandatory download-tracking trigger, and the image
// download for id, caching both the metadata and the image bytes before returning the Blob. A 404
// detail response is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(ctx context.Context, id, imgKey, metaKey string) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: rate limit wait: %w", err)
	}

	detailURL := baseURL + "/photos/" + url.PathEscape(id)
	var ph photo
	if err := p.client.GetJSONHeaders(ctx, detailURL, p.authHeader(), &ph); err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return assetcore.Blob{}, fmt.Errorf("unsplash: photo %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("unsplash: fetch detail: %w", err)
	}

	if ph.Links.DownloadLocation != "" {
		if err := p.limiter.Wait(ctx); err != nil {
			return assetcore.Blob{}, fmt.Errorf("unsplash: rate limit wait: %w", err)
		}
		if _, err := p.client.GetBytesHeaders(ctx, ph.Links.DownloadLocation, p.authHeader()); err != nil {
			return assetcore.Blob{}, fmt.Errorf("unsplash: trigger download: %w", err)
		}
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: rate limit wait: %w", err)
	}
	imgData, err := p.client.GetBytesHeaders(ctx, ph.Urls.Full, p.authHeader())
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: fetch image: %w", err)
	}

	metaData, err := json.Marshal(ph)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(imgKey, imgData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("unsplash: cache put %s: %w", imgKey, err)
	}

	return blobFor(id, ph, imgData), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) image, deriving the content type
// and filename extension from p's full-resolution image URL.
func blobFor(id string, p photo, content []byte) assetcore.Blob {
	contentType, ext := contentTypeForURL(p.Urls.Full)
	return assetcore.Blob{
		Asset:       asset(p),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

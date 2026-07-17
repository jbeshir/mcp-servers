// Package pixabay serves the Pixabay API (https://pixabay.com/api/) as an assetcore.PhotoProvider,
// searching Pixabay's photo catalogue and fetching the underlying bytes on demand. Every request is
// authenticated with the caller-supplied API key as a query parameter; the package never reads the
// environment itself.
package pixabay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Pixabay provider.
const providerName = "pixabay"

// baseURL is the Pixabay API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://pixabay.com"

// defaultPage is the page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// minPerPage is the smallest per_page value Pixabay's search endpoint accepts.
const minPerPage = 3

// searchResult is the Pixabay search envelope returned by GET /api/, shared by the paged search query
// and the single-hit id lookup used by Fetch.
type searchResult struct {
	Total     int   `json:"total"`
	TotalHits int   `json:"totalHits"`
	Hits      []hit `json:"hits"`
}

// hit is a single Pixabay image record. fullHDURL and imageURL also exist on the upstream payload but
// require an elevated API key tier, so they are omitted here rather than relied upon.
type hit struct {
	ID            int    `json:"id"`
	PageURL       string `json:"pageURL"`
	Tags          string `json:"tags"`
	PreviewURL    string `json:"previewURL"`
	LargeImageURL string `json:"largeImageURL"`
	User          string `json:"user"`
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

// license builds the assetcore.License for a Pixabay image credited to user. Pixabay has no SPDX-style
// license identifier of its own and requires no attribution, though crediting the photographer is
// carried in Attribution as courtesy.
func license(user string) assetcore.License {
	return assetcore.License{
		SPDX:                "",
		Name:                "Pixabay Content License",
		URL:                 "https://pixabay.com/service/license-summary/",
		Attribution:         "Image by " + user + " on Pixabay",
		RequiresAttribution: false,
	}
}

// title picks the display title for h: its full tags string, trimmed, falling back to its numeric id
// when tags is empty or blank. A partial (first-tag-only) title collides too easily with other hits
// sharing a common leading tag, so the whole string is used to keep (Source, Title) distinctive.
func title(h hit) string {
	trimmed := strings.TrimSpace(h.Tags)
	if trimmed == "" {
		return strconv.Itoa(h.ID)
	}
	return trimmed
}

// tags splits h.Tags, Pixabay's comma-space-separated keyword string, into individual tags. An empty
// tags string yields a nil slice.
func tags(h hit) []string {
	if h.Tags == "" {
		return nil
	}
	return strings.Split(h.Tags, ", ")
}

// asset builds the assetcore.Asset for a Pixabay image record.
func asset(h hit) assetcore.Asset {
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, strconv.Itoa(h.ID)),
		Kind:       assetcore.KindPhoto,
		Title:      title(h),
		Tags:       tags(h),
		Source:     h.User,
		License:    license(h.User),
		LandingURL: h.PageURL,
		PreviewURL: h.PreviewURL,
	}
}

// Provider satisfies assetcore.PhotoProvider.
var _ assetcore.PhotoProvider = (*Provider)(nil)

// Provider serves the Pixabay API as an assetcore.PhotoProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
	apiKey  string
}

// New returns a Pixabay provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched image bytes, and apiKey to authenticate every
// request against the Pixabay API.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, apiKey string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, apiKey: apiKey}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves photos.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindPhoto }

// Search queries the Pixabay image search endpoint for opts.Query, honouring opts.Cursor as a page
// number (default 1) and opts.Limit as the page size. Pixabay has no source facet, so opts.Sources is
// ignored, matching Openverse's handling of the same field.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("pixabay: rate limit wait: %w", err)
	}

	page := defaultPage
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		page = n
	}
	perPage := assetcore.ClampLimit(opts.Limit)
	// Pixabay's API rejects per_page < 3 with a 400, so raise it to the minimum it accepts even
	// when a caller asked for fewer results.
	if perPage < minPerPage {
		perPage = minPerPage
	}

	q := url.Values{
		"key":        {p.apiKey},
		"q":          {opts.Query},
		"image_type": {"photo"},
		"page":       {strconv.Itoa(page)},
		"per_page":   {strconv.Itoa(perPage)},
	}
	reqURL := baseURL + "/api/?" + q.Encode()

	var result searchResult
	if err := p.client.GetJSON(ctx, reqURL, &result); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("pixabay: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(result.Hits))
	for _, h := range result.Hits {
		assets = append(assets, asset(h))
	}

	// Pixabay caps reachable results at totalHits (itself capped at 500 by the API), so stop
	// advancing the cursor once this page has reached or passed it.
	var nextCursor string
	if page*perPage < result.TotalHits {
		nextCursor = strconv.Itoa(page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// imageCacheKey and metaCacheKey return the on-disk cache keys for id's image bytes and detail
// metadata respectively, namespaced by provider so the two never collide.
func imageCacheKey(id string) string { return cache.Key(providerName, "img", id) }
func metaCacheKey(id string) string  { return cache.Key(providerName, "meta", id) }

// Fetch returns the image identified by the provider-local numeric id (as a string), checking the
// on-disk cache for both the image bytes and the detail metadata (license, title, landing/preview
// URLs) before making any network call. On a hit for both entries, the Blob is rebuilt from cache
// alone, with zero HTTP requests. On a miss, it looks up the hit by id, downloads the image bytes from
// its largeImageURL, and caches both before returning. Pixabay has no detail endpoint, so the lookup is
// a search request scoped to a single id; an empty hits list is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, _ assetcore.PhotoFetchOpts) (assetcore.Blob, error) {
	imgKey := imageCacheKey(id)
	metaKey := metaCacheKey(id)

	blob, ok, err := p.cachedBlob(id, imgKey, metaKey)
	if err != nil {
		return assetcore.Blob{}, err
	}
	if ok {
		return blob, nil
	}

	return p.fetchAndCache(ctx, id, imgKey, metaKey)
}

// cachedBlob rebuilds the Blob for id from the on-disk cache when both its image bytes and detail
// metadata are present, returning ok=false with a zero Blob when either entry is missing.
func (p *Provider) cachedBlob(id, imgKey, metaKey string) (assetcore.Blob, bool, error) {
	imgData, imgHit, err := p.cache.Get(imgKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("pixabay: cache get %s: %w", imgKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("pixabay: cache get %s: %w", metaKey, err)
	}
	if !imgHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var h hit
	if err := json.Unmarshal(metaData, &h); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("pixabay: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, h, imgData), true, nil
}

// fetchAndCache looks up id's hit, downloads its image bytes from largeImageURL (never hotlinked; the
// bytes are stored on disk so a warm Fetch satisfies Pixabay's 24-hour cache requirement without a
// repeat request), caches both the metadata and the image bytes, and returns the Blob. An empty hits
// list is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(ctx context.Context, id, imgKey, metaKey string) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: rate limit wait: %w", err)
	}

	q := url.Values{
		"key":        {p.apiKey},
		"id":         {id},
		"image_type": {"photo"},
	}
	lookupURL := baseURL + "/api/?" + q.Encode()

	var result searchResult
	if err := p.client.GetJSON(ctx, lookupURL, &result); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: fetch detail: %w", err)
	}
	if len(result.Hits) == 0 {
		return assetcore.Blob{}, fmt.Errorf("pixabay: image %q: %w", id, assetcore.ErrNotFound)
	}
	h := result.Hits[0]

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: rate limit wait: %w", err)
	}
	imgData, err := p.client.GetBytes(ctx, h.LargeImageURL)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: fetch image: %w", err)
	}

	metaData, err := json.Marshal(h)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(imgKey, imgData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("pixabay: cache put %s: %w", imgKey, err)
	}

	return blobFor(id, h, imgData), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) image, deriving the content type
// and filename extension from h's largeImageURL.
func blobFor(id string, h hit, content []byte) assetcore.Blob {
	contentType, ext := contentTypeForURL(h.LargeImageURL)
	return assetcore.Blob{
		Asset:       asset(h),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

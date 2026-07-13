// Package iconify serves the Iconify remote icon API (api.iconify.design) as an assetcore.IconProvider,
// searching across every collection Iconify hosts and rendering individual icons to standalone SVG via
// the upstream .svg endpoint.
package iconify

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Iconify remote icon provider.
const providerName = "iconify"

// baseURL is the Iconify API base, overridden in tests to point at an httptest server.
var baseURL = "https://api.iconify.design"

// searchPageFloor is the smallest page size the Iconify search endpoint honours; a request for fewer
// results still returns up to this many.
const searchPageFloor = 32

var _ assetcore.IconProvider = (*Provider)(nil)

// Provider serves the Iconify remote icon API as an assetcore.IconProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache

	collectionsOnce sync.Once
	collections     map[string]assetcore.License
}

// New returns an Iconify provider using client for HTTP requests, limiter to pace requests to the
// Iconify API, and cache to avoid re-downloading previously fetched SVGs.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves icons.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIcon }

// searchResponse is the JSON shape of a GET /search response.
type searchResponse struct {
	Icons       []string                  `json:"icons"`
	Total       int                       `json:"total"`
	Limit       int                       `json:"limit"`
	Start       int                       `json:"start"`
	Collections map[string]collectionInfo `json:"collections"`
}

// collectionInfo is the per-prefix metadata embedded in a search response and returned in full by
// GET /collections.
type collectionInfo struct {
	Name    string      `json:"name"`
	License licenseInfo `json:"license"`
}

// licenseInfo is the Iconify license shape, mapped onto assetcore.License.
type licenseInfo struct {
	Title string `json:"title"`
	SPDX  string `json:"spdx"`
	URL   string `json:"url"`
}

// toAsset maps the Iconify license shape onto assetcore.License. Iconify licenses carry no attribution
// requirement of their own.
func (l licenseInfo) toAsset() assetcore.License {
	return assetcore.License{SPDX: l.SPDX, Name: l.Title, URL: l.URL}
}

// Search finds icons across every Iconify collection matching opts.Query, honouring opts.Sources as a
// filter over collection prefixes. Pagination follows the Iconify search endpoint's start-offset
// cursor, whose effective page size is floored at searchPageFloor.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	start := 0
	if opts.Cursor != "" {
		n, err := strconv.Atoi(opts.Cursor)
		if err != nil {
			return assetcore.SearchResult{}, fmt.Errorf("iconify: invalid cursor %q: %w", opts.Cursor, err)
		}
		start = n
	}

	limit := assetcore.ClampLimit(opts.Limit)
	if limit < searchPageFloor {
		limit = searchPageFloor
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("iconify: rate limit wait: %w", err)
	}

	q := url.Values{}
	q.Set("query", opts.Query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("start", strconv.Itoa(start))

	var resp searchResponse
	if err := p.client.GetJSON(ctx, baseURL+"/search?"+q.Encode(), &resp); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("iconify: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(resp.Icons))
	for _, entry := range resp.Icons {
		prefix, name, ok := strings.Cut(entry, ":")
		if !ok || !opts.Sources.Allows(prefix) {
			continue
		}
		assets = append(assets, p.asset(prefix, name, p.licenseFor(ctx, prefix, resp.Collections)))
	}

	var next string
	if n := len(resp.Icons); n > 0 && start+n < resp.Total {
		next = strconv.Itoa(start + n)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: next}, nil
}

// Fetch downloads the standalone SVG for the icon identified by the provider-local id "<prefix>/<name>",
// honouring the colour and size in opts. Results are cached on disk keyed by id and render parameters.
// A malformed id or an unknown icon is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, opts assetcore.IconFetchOpts) (assetcore.Blob, error) {
	prefix, name, ok := strings.Cut(id, "/")
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("%w: malformed icon id %q", assetcore.ErrNotFound, id)
	}

	key := cacheKey(id, opts.Color, opts.Size)
	if data, hit, err := p.cache.Get(key); err != nil {
		return assetcore.Blob{}, fmt.Errorf("iconify: cache get: %w", err)
	} else if hit {
		return p.blob(ctx, prefix, name, data), nil
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("iconify: rate limit wait: %w", err)
	}

	data, err := p.fetchSVG(ctx, prefix, name, opts)
	if err != nil {
		return assetcore.Blob{}, err
	}

	if err := p.cache.Put(key, data); err != nil {
		return assetcore.Blob{}, fmt.Errorf("iconify: cache put: %w", err)
	}

	return p.blob(ctx, prefix, name, data), nil
}

// fetchSVG downloads the raw SVG bytes for prefix/name from the upstream .svg endpoint, mapping a 404
// onto assetcore.ErrNotFound.
func (p *Provider) fetchSVG(ctx context.Context, prefix, name string, opts assetcore.IconFetchOpts) ([]byte, error) {
	q := url.Values{}
	if opts.Color != "" {
		q.Set("color", opts.Color)
	}
	if opts.Size != 0 {
		q.Set("height", strconv.Itoa(opts.Size))
	}

	svgURL := baseURL + "/" + prefix + "/" + name + ".svg"
	if encoded := q.Encode(); encoded != "" {
		svgURL += "?" + encoded
	}

	data, err := p.client.GetBytes(ctx, svgURL)
	if err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: icon %q/%q", assetcore.ErrNotFound, prefix, name)
		}

		return nil, fmt.Errorf("iconify: fetch %q/%q: %w", prefix, name, err)
	}

	return data, nil
}

// cacheKey builds the on-disk cache key for a rendered icon, incorporating the render parameters so
// distinct colour/size combinations are cached separately.
func cacheKey(local, color string, size int) string {
	return cache.Key(providerName, local, "color="+color, "h="+strconv.Itoa(size))
}

// blob builds the Blob for a successfully fetched icon's SVG bytes, resolving its license from the
// in-process collections cache.
func (p *Provider) blob(ctx context.Context, prefix, name string, data []byte) assetcore.Blob {
	return assetcore.Blob{
		Asset:       p.asset(prefix, name, p.licenseFor(ctx, prefix, nil)),
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}
}

// asset builds the assetcore.Asset for an icon, stamping its composite id and resolved license.
func (p *Provider) asset(prefix, name string, license assetcore.License) assetcore.Asset {
	return assetcore.Asset{
		Source:  prefix,
		ID:      assetcore.AssetID(providerName, prefix+"/"+name),
		Kind:    assetcore.KindIcon,
		Title:   name,
		License: license,
	}
}

// licenseFor resolves prefix's license from fromSearch (a search response's own collections map) when
// present, falling back to the in-process /collections cache.
func (p *Provider) licenseFor(
	ctx context.Context,
	prefix string,
	fromSearch map[string]collectionInfo,
) assetcore.License {
	if info, ok := fromSearch[prefix]; ok {
		return info.License.toAsset()
	}

	p.ensureCollections(ctx)
	return p.collections[prefix]
}

// ensureCollections lazily fetches the full Iconify collections index once, caching each prefix's
// license for use as a Search/Fetch fallback when a response omits it. A fetch failure degrades to an
// empty license set rather than failing the caller.
func (p *Provider) ensureCollections(ctx context.Context) {
	p.collectionsOnce.Do(func() {
		p.collections = map[string]assetcore.License{}

		if err := p.limiter.Wait(ctx); err != nil {
			return
		}

		var resp map[string]collectionInfo
		if err := p.client.GetJSON(ctx, baseURL+"/collections", &resp); err != nil {
			return
		}

		for prefix, info := range resp {
			p.collections[prefix] = info.License.toAsset()
		}
	})
}

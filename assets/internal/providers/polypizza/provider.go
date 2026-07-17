// Package polypizza serves the Poly Pizza API (https://api.poly.pizza) as an assetcore.ModelProvider,
// searching its catalogue of Creative-Commons-licensed 3D models and fetching the underlying .glb bytes
// on demand. Every request against api.poly.pizza is authenticated with the caller-supplied API key via
// the x-auth-token header; the package never reads the environment itself. The CDN URL a model's
// Download field points at is fetched without that header.
package polypizza

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Poly Pizza provider.
const providerName = "polypizza"

// baseURL is the Poly Pizza API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://api.poly.pizza/v1.1"

// defaultPage is the page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// searchEnvelope is the top-level shape of a GET /search/{query} response.
type searchEnvelope struct {
	Total   int     `json:"total"`
	Results []model `json:"results"`
}

// creator identifies a model's uploader.
type creator struct {
	Username string `json:"Username"`
}

// model is a single Poly Pizza model record, shared by the search envelope and the detail endpoint
// (GET /model/{id}). Field names are PascalCase, matching Poly Pizza's wire format exactly, right down
// to the space in "Tri Count".
type model struct {
	ID          string  `json:"ID"`
	Title       string  `json:"Title"`
	Download    string  `json:"Download"`
	Thumbnail   string  `json:"Thumbnail"`
	Creator     creator `json:"Creator"`
	Licence     string  `json:"Licence"`
	Attribution string  `json:"Attribution"`
	TriCount    int     `json:"Tri Count"`
	Animated    bool    `json:"Animated"`
}

// cc0License is stamped onto every model whose Licence is a CC0 dedication.
var cc0License = assetcore.License{
	SPDX:                "CC0-1.0",
	Name:                "Creative Commons Zero v1.0 Universal",
	URL:                 "https://creativecommons.org/publicdomain/zero/1.0/",
	RequiresAttribution: false,
}

// license builds the assetcore.License for a Poly Pizza model, parsing m.Licence. A "CC0"-prefixed
// licence maps to cc0License. Anything else is treated as a member of the CC-BY family: its SPDX
// identifier is derived from the licence token itself (e.g. "CC-BY 3.0" -> "CC-BY-3.0"), and its
// attribution is m.Attribution, falling back to a generated credit line when Poly Pizza left it empty.
// An unrecognized licence string (one not beginning "CC-") still requires attribution but carries no
// SPDX identifier.
func license(m model) assetcore.License {
	licence := strings.ToUpper(strings.TrimSpace(m.Licence))
	if strings.HasPrefix(licence, "CC0") {
		return cc0License
	}

	attribution := m.Attribution
	if attribution == "" {
		attribution = "Model by " + m.Creator.Username + " on Poly Pizza"
	}

	var spdx string
	if strings.HasPrefix(licence, "CC-") {
		spdx = strings.ReplaceAll(licence, " ", "-")
	}

	return assetcore.License{
		SPDX:                spdx,
		Name:                m.Licence,
		Attribution:         attribution,
		RequiresAttribution: true,
	}
}

// asset builds the assetcore.Asset for a Poly Pizza model record.
func asset(m model) assetcore.Asset {
	title := m.Title
	if title == "" {
		title = m.ID
	}
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, m.ID),
		Kind:       assetcore.KindModel,
		Title:      title,
		Source:     m.Creator.Username,
		License:    license(m),
		LandingURL: "https://poly.pizza/m/" + m.ID,
		PreviewURL: m.Thumbnail,
	}
}

// Provider satisfies assetcore.ModelProvider.
var _ assetcore.ModelProvider = (*Provider)(nil)

// Provider serves the Poly Pizza API as an assetcore.ModelProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
	apiKey  string
}

// New returns a Poly Pizza provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched model bytes, and apiKey to authenticate every
// request against api.poly.pizza.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, apiKey string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, apiKey: apiKey}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves 3D models.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindModel }

// authHeader builds the header set required by every api.poly.pizza request: the x-auth-token carrying
// the caller-supplied API key.
func (p *Provider) authHeader() http.Header {
	return http.Header{"X-Auth-Token": {p.apiKey}}
}

// Search queries the Poly Pizza keyword search endpoint for opts.Query, honouring opts.Cursor as a page
// number (default 1) and opts.Limit as the page size.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("polypizza: rate limit wait: %w", err)
	}

	page := defaultPage
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		page = n
	}
	limit := assetcore.ClampLimit(opts.Limit)

	q := url.Values{
		"Limit": {strconv.Itoa(limit)},
		"page":  {strconv.Itoa(page)},
	}
	reqURL := baseURL + "/search/" + url.PathEscape(opts.Query) + "?" + q.Encode()

	var env searchEnvelope
	if err := p.client.GetJSONHeaders(ctx, reqURL, p.authHeader(), &env); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("polypizza: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(env.Results))
	for _, m := range env.Results {
		assets = append(assets, asset(m))
	}

	var nextCursor string
	if page*limit < env.Total {
		nextCursor = strconv.Itoa(page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// glbCacheKey and metaCacheKey return the on-disk cache keys for id's model bytes and detail metadata
// respectively, namespaced by provider so the two never collide.
func glbCacheKey(id string) string  { return cache.Key(providerName, "glb", id) }
func metaCacheKey(id string) string { return cache.Key(providerName, "meta", id) }

// Fetch returns the model identified by the provider-local id, checking the on-disk cache for both the
// .glb bytes and the detail metadata (license, title, landing/preview URLs) before making any network
// call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP requests. On a
// miss, it performs the detail GET, downloads the .glb bytes from the model's Download URL, and caches
// both before returning. Poly Pizza serves each model as a single self-contained .glb, so opts is
// unused. A 404 detail response is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(ctx context.Context, id string, _ assetcore.ModelFetchOpts) (assetcore.Blob, error) {
	glbKey := glbCacheKey(id)
	metaKey := metaCacheKey(id)

	blob, hit, err := p.cachedBlob(glbKey, metaKey)
	if err != nil {
		return assetcore.Blob{}, err
	}
	if hit {
		return blob, nil
	}

	return p.fetchAndCache(ctx, id, glbKey, metaKey)
}

// cachedBlob rebuilds the Blob for a model from the on-disk cache when both its .glb bytes and detail
// metadata are present, returning hit=false with a zero Blob when either entry is missing.
func (p *Provider) cachedBlob(glbKey, metaKey string) (assetcore.Blob, bool, error) {
	glbData, glbHit, err := p.cache.Get(glbKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("polypizza: cache get %s: %w", glbKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("polypizza: cache get %s: %w", metaKey, err)
	}
	if !glbHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var m model
	if err := json.Unmarshal(metaData, &m); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("polypizza: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(m, glbData), true, nil
}

// fetchAndCache performs the detail lookup and the .glb download for id, caching both the metadata and
// the model bytes before returning the Blob. The detail GET is authenticated with the x-auth-token
// header; the .glb download, against the model's own Download URL (Poly Pizza's CDN), is not. A 404
// detail response is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(ctx context.Context, id, glbKey, metaKey string) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: rate limit wait: %w", err)
	}

	detailURL := baseURL + "/model/" + url.PathEscape(id)
	var m model
	if err := p.client.GetJSONHeaders(ctx, detailURL, p.authHeader(), &m); err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return assetcore.Blob{}, fmt.Errorf("polypizza: model %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("polypizza: fetch detail: %w", err)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: rate limit wait: %w", err)
	}
	glbData, err := p.client.GetBytes(ctx, m.Download)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: download glb: %w", err)
	}

	metaData, err := json.Marshal(m)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(glbKey, glbData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("polypizza: cache put %s: %w", glbKey, err)
	}

	return blobFor(m, glbData), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) model.
func blobFor(m model, content []byte) assetcore.Blob {
	return assetcore.Blob{
		Asset:       asset(m),
		Content:     content,
		Filename:    m.ID + ".glb",
		ContentType: "model/gltf-binary",
	}
}

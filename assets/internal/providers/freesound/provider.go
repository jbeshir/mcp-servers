// Package freesound serves the Freesound APIv2 (https://freesound.org/docs/api) as an
// assetcore.AudioProvider, searching its catalogue of Creative-Commons-licensed sounds and fetching
// audio on demand. Every request against freesound.org is authenticated with the caller-supplied API key
// via an "Authorization: Token <key>" header; the package never reads the environment itself. Fetch
// downloads Freesound's high-quality PREVIEW (preview-hq-mp3 / preview-hq-ogg), not the original master
// file: the original requires an OAuth2 access token and is out of scope for this token-auth provider.
package freesound

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

// providerName is the stable registry key for the Freesound provider.
const providerName = "freesound"

// baseURL is the Freesound API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://freesound.org/apiv2"

// defaultPage is the search page number used when a SearchOpts.Cursor is empty or unparseable.
const defaultPage = 1

// searchFields lists the sound fields requested from both the search and by-id detail endpoints; it
// covers exactly the fields asset/license/Fetch consume.
const searchFields = "id,name,username,license,previews"

// cc0SPDX is the SPDX identifier for the Creative Commons Zero deed, the one license Freesound serves
// that requires no attribution.
const cc0SPDX = "CC0-1.0"

// formatOGG is the AudioFetchOpts.Format value (and cache-key format selector) selecting the OGG preview;
// any other value resolves to the mp3 default.
const formatOGG = "ogg"

// searchEnvelope is the top-level shape of a GET /search/text/ response.
type searchEnvelope struct {
	Count   int     `json:"count"`
	Next    string  `json:"next"`
	Results []sound `json:"results"`
}

// previews is the set of preview-quality audio URLs carried by a sound record.
type previews struct {
	PreviewHQMP3 string `json:"preview-hq-mp3"`
	PreviewHQOGG string `json:"preview-hq-ogg"`
	PreviewLQMP3 string `json:"preview-lq-mp3"`
}

// sound is a single Freesound sound record, shared by the search envelope and the by-id detail lookup
// used during Fetch.
type sound struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Username string   `json:"username"`
	License  string   `json:"license"` // a Creative Commons deed URL, not a human-readable name
	Previews previews `json:"previews"`
}

// normalizeLicenseURL strips a Creative Commons deed URL's scheme and any trailing slash and lower-cases
// it, so http/https and trailing-slash variants of the same deed match the same case in spdxForDeedPath.
func normalizeLicenseURL(deedURL string) string {
	s := strings.TrimPrefix(deedURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.ToLower(strings.TrimSuffix(s, "/"))
}

// spdxForDeedPath maps a normalized Creative Commons deed URL path to its SPDX identifier. Freesound's
// license set is narrower than Jamendo's: Attribution, Attribution-NonCommercial (4.0 and legacy 3.0),
// CC0, and the legacy Sampling+ deed. An unrecognized path returns "".
func spdxForDeedPath(deedPath string) string {
	switch deedPath {
	case "creativecommons.org/licenses/by/4.0":
		return "CC-BY-4.0"
	case "creativecommons.org/licenses/by-nc/4.0":
		return "CC-BY-NC-4.0"
	case "creativecommons.org/licenses/by/3.0":
		return "CC-BY-3.0"
	case "creativecommons.org/licenses/by-nc/3.0":
		return "CC-BY-NC-3.0"
	case "creativecommons.org/publicdomain/zero/1.0":
		return cc0SPDX
	default:
		return ""
	}
}

// humanLicenseName returns a short human-readable label for spdx: the CC0-specific name for CC0-1.0, a
// derived "CC BY ..." label for other recognized SPDX identifiers, or "Creative Commons" for an
// unrecognized (empty) one.
func humanLicenseName(spdx string) string {
	switch spdx {
	case cc0SPDX:
		return "Creative Commons Zero v1.0 Universal"
	case "":
		return "Creative Commons"
	default:
		return strings.ReplaceAll(spdx, "-", " ")
	}
}

// license builds the assetcore.License for a Freesound sound. CC0 requires no attribution; every other
// license (including the unrecognized/legacy Sampling+ deed) requires attribution to both the uploader
// and the sound's title.
func license(s sound) assetcore.License {
	spdx := spdxForDeedPath(normalizeLicenseURL(s.License))
	if spdx == cc0SPDX {
		return assetcore.License{
			SPDX: spdx,
			Name: humanLicenseName(spdx),
			URL:  s.License,
		}
	}

	return assetcore.License{
		SPDX:                spdx,
		Name:                humanLicenseName(spdx),
		URL:                 s.License,
		Attribution:         s.Username + " — " + s.Name + " (via Freesound)",
		RequiresAttribution: true,
	}
}

// asset builds the assetcore.Asset for a Freesound sound record.
func asset(s sound) assetcore.Asset {
	id := strconv.Itoa(s.ID)
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, id),
		Kind:       assetcore.KindAudio,
		Title:      s.Name,
		Source:     s.Username,
		License:    license(s),
		LandingURL: "https://freesound.org/s/" + id + "/",
		PreviewURL: s.Previews.PreviewLQMP3,
	}
}

// Provider satisfies assetcore.AudioProvider.
var _ assetcore.AudioProvider = (*Provider)(nil)

// Provider serves the Freesound API as an assetcore.AudioProvider.
type Provider struct {
	client  *httpx.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
	apiKey  string
}

// New returns a Freesound provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched preview bytes, and apiKey to authenticate every
// request against freesound.org.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, apiKey string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, apiKey: apiKey}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves audio.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindAudio }

// authHeader builds the header set required by every freesound.org request: the API key as a Token
// credential.
func (p *Provider) authHeader() http.Header {
	return http.Header{"Authorization": {"Token " + p.apiKey}}
}

// Search queries the Freesound text search endpoint for opts.Query, honouring opts.Cursor as a one-based
// page number and opts.Limit as the page size.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("freesound: rate limit wait: %w", err)
	}

	page := defaultPage
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		page = n
	}
	limit := assetcore.ClampLimit(opts.Limit)

	q := url.Values{
		"query":     {opts.Query},
		"page":      {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(limit)},
		"fields":    {searchFields},
	}
	reqURL := baseURL + "/search/text/?" + q.Encode()

	var env searchEnvelope
	if err := p.client.GetJSONHeaders(ctx, reqURL, p.authHeader(), &env); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("freesound: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(env.Results))
	for _, s := range env.Results {
		assets = append(assets, asset(s))
	}

	var nextCursor string
	if env.Next != "" {
		nextCursor = strconv.Itoa(page + 1)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// resolveFormat maps an AudioFetchOpts.Format to the cache-key format selector, the Blob's Content-Type,
// and its file extension. Any value other than "ogg" (including "" and "mp3") resolves to the mp3
// default.
func resolveFormat(format string) (sel, contentType, ext string) {
	if format == formatOGG {
		return formatOGG, "audio/ogg", ".ogg"
	}
	return "mp3", "audio/mpeg", ".mp3"
}

// previewURL returns s's HQ preview URL for the sel format selector ("mp3" or "ogg").
func previewURL(s sound, sel string) string {
	if sel == formatOGG {
		return s.Previews.PreviewHQOGG
	}
	return s.Previews.PreviewHQMP3
}

// metaCacheKey returns the on-disk cache key for id's detail metadata, namespaced by provider so it
// never collides with an audio bytes cache entry.
func metaCacheKey(id string) string { return cache.Key(providerName, "meta", id) }

// audioCacheKey returns the on-disk cache key for id's format-encoded audio bytes, namespaced by
// provider and format so an mp3 and an ogg download for the same sound never collide.
func audioCacheKey(format, id string) string { return cache.Key(providerName, "audio", format, id) }

// Fetch returns the audio clip identified by the provider-local id, encoded per opts.Format, checking
// the on-disk cache for both the format-encoded audio bytes and the detail metadata before making any
// network call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP requests.
// On a miss, it performs the detail lookup, downloads the HQ preview bytes (never the original master —
// that requires OAuth2 and is out of scope), and caches both before returning.
func (p *Provider) Fetch(ctx context.Context, id string, opts assetcore.AudioFetchOpts) (assetcore.Blob, error) {
	sel, contentType, ext := resolveFormat(opts.Format)
	metaKey := metaCacheKey(id)
	audioKey := audioCacheKey(sel, id)

	blob, hit, err := p.cachedBlob(id, audioKey, metaKey, contentType, ext)
	if err != nil {
		return assetcore.Blob{}, err
	}
	if hit {
		return blob, nil
	}

	return p.fetchAndCache(ctx, id, sel, audioKey, metaKey, contentType, ext)
}

// cachedBlob rebuilds the Blob for id from the on-disk cache when both its format-encoded audio bytes and
// detail metadata are present, returning hit=false with a zero Blob when either entry is missing.
func (p *Provider) cachedBlob(id, audioKey, metaKey, contentType, ext string) (assetcore.Blob, bool, error) {
	audioData, audioHit, err := p.cache.Get(audioKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("freesound: cache get %s: %w", audioKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("freesound: cache get %s: %w", metaKey, err)
	}
	if !audioHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var s sound
	if err := json.Unmarshal(metaData, &s); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("freesound: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, s, audioData, contentType, ext), true, nil
}

// fetchAndCache performs the by-id detail lookup and the HQ preview download for id in the sel format,
// caching both the metadata and the audio bytes before returning the Blob. A 404 detail response, or a
// sound with no preview URL for the requested format, is reported as assetcore.ErrNotFound.
func (p *Provider) fetchAndCache(
	ctx context.Context, id, sel, audioKey, metaKey, contentType, ext string,
) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: rate limit wait: %w", err)
	}

	dq := url.Values{"fields": {searchFields}}
	detailURL := baseURL + "/sounds/" + url.PathEscape(id) + "/?" + dq.Encode()
	var s sound
	if err := p.client.GetJSONHeaders(ctx, detailURL, p.authHeader(), &s); err != nil {
		if httpx.IsStatus(err, http.StatusNotFound) {
			return assetcore.Blob{}, fmt.Errorf("freesound: sound %q: %w", id, assetcore.ErrNotFound)
		}
		return assetcore.Blob{}, fmt.Errorf("freesound: fetch detail: %w", err)
	}

	pURL := previewURL(s, sel)
	if pURL == "" {
		return assetcore.Blob{}, fmt.Errorf("freesound: sound %q: %w", id, assetcore.ErrNotFound)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: rate limit wait: %w", err)
	}
	// The preview CDN is public, but the Token header is sent defensively on every request regardless —
	// this downloads Freesound's HQ preview, never the OAuth2-gated original master file.
	audioData, err := p.client.GetBytesHeaders(ctx, pURL, p.authHeader())
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: download preview: %w", err)
	}

	metaData, err := json.Marshal(s)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(audioKey, audioData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("freesound: cache put %s: %w", audioKey, err)
	}

	return blobFor(id, s, audioData, contentType, ext), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) sound.
func blobFor(id string, s sound, content []byte, contentType, ext string) assetcore.Blob {
	return assetcore.Blob{
		Asset:       asset(s),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

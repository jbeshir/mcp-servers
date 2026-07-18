// Package jamendo serves the Jamendo v3.0 API (https://developer.jamendo.com) as an
// assetcore.AudioProvider, searching its catalogue of Creative-Commons-licensed music tracks and
// fetching the audio file on demand. Every request against api.jamendo.com is authenticated with the
// caller-supplied client_id as a query parameter (Jamendo does not accept it as a header); the package
// never reads the environment itself.
package jamendo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// providerName is the stable registry key for the Jamendo provider.
const providerName = "jamendo"

// baseURL is the Jamendo API root. A package-level var so tests can point it at an httptest server.
var baseURL = "https://api.jamendo.com/v3.0"

// defaultOffset is the search offset used when a SearchOpts.Cursor is empty or unparseable.
const defaultOffset = 0

// dlFormatMP3 and dlFormatOGG are the Jamendo audioformat/audiodlformat selectors for the mp3 and ogg
// audio encodings.
const (
	dlFormatMP3 = "mp32"
	dlFormatOGG = "ogg"
)

// tracksEnvelope is the top-level shape of a GET /tracks/ response, shared by search and the by-id
// detail lookup used during Fetch.
type tracksEnvelope struct {
	Results []track `json:"results"`
}

// track is a single Jamendo track record, shared by the search envelope and the by-id detail lookup.
type track struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArtistName    string `json:"artist_name"`
	Audio         string `json:"audio"`
	AudioDownload string `json:"audiodownload"`
	LicenseCCURL  string `json:"license_ccurl"`
	ShareURL      string `json:"shareurl"`
}

// normalizeLicenseURL strips a Creative Commons deed URL's scheme and any trailing slash and lower-cases
// it, so http/https and trailing-slash variants of the same deed match the same case in spdxForDeedPath.
func normalizeLicenseURL(deedURL string) string {
	s := strings.TrimPrefix(deedURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.ToLower(strings.TrimSuffix(s, "/"))
}

// spdxForDeedPath maps a normalized Creative Commons deed URL path to its SPDX identifier. Jamendo
// licenses its whole catalogue under CC 3.0 (unported); there is no CC0 track. An unrecognized path (a
// legacy Sampling+/Free Art License deed, or anything else) returns "".
func spdxForDeedPath(deedPath string) string {
	switch deedPath {
	case "creativecommons.org/licenses/by/3.0":
		return "CC-BY-3.0"
	case "creativecommons.org/licenses/by-sa/3.0":
		return "CC-BY-SA-3.0"
	case "creativecommons.org/licenses/by-nc/3.0":
		return "CC-BY-NC-3.0"
	case "creativecommons.org/licenses/by-nc-sa/3.0":
		return "CC-BY-NC-SA-3.0"
	case "creativecommons.org/licenses/by-nd/3.0":
		return "CC-BY-ND-3.0"
	case "creativecommons.org/licenses/by-nc-nd/3.0":
		return "CC-BY-NC-ND-3.0"
	default:
		return ""
	}
}

// humanLicenseName returns a short human-readable label derived from spdx, or "Creative Commons" for an
// unrecognized (empty) spdx identifier.
func humanLicenseName(spdx string) string {
	if spdx == "" {
		return "Creative Commons"
	}
	return strings.ReplaceAll(spdx, "-", " ")
}

// license builds the assetcore.License for a Jamendo track. Jamendo's catalogue carries no CC0 track,
// so every license requires attribution, including the legacy/unrecognized deed URLs that carry no SPDX
// identifier.
func license(t track) assetcore.License {
	spdx := spdxForDeedPath(normalizeLicenseURL(t.LicenseCCURL))
	return assetcore.License{
		SPDX:                spdx,
		Name:                humanLicenseName(spdx),
		URL:                 t.LicenseCCURL,
		Attribution:         t.ArtistName + " — " + t.Name + " (via Jamendo)",
		RequiresAttribution: true,
	}
}

// asset builds the assetcore.Asset for a Jamendo track record.
func asset(t track) assetcore.Asset {
	return assetcore.Asset{
		ID:         assetcore.AssetID(providerName, t.ID),
		Kind:       assetcore.KindAudio,
		Title:      t.Name,
		Source:     t.ArtistName,
		License:    license(t),
		LandingURL: t.ShareURL,
		PreviewURL: t.Audio,
	}
}

// Provider satisfies assetcore.AudioProvider.
var _ assetcore.AudioProvider = (*Provider)(nil)

// Provider serves the Jamendo API as an assetcore.AudioProvider.
type Provider struct {
	client   *httpx.Client
	limiter  *ratelimit.Limiter
	cache    *cache.Cache
	clientID string
}

// New returns a Jamendo provider using client for HTTP requests, limiter to pace outbound requests,
// cache to avoid re-downloading previously fetched track bytes, and clientID to authenticate every
// request against api.jamendo.com.
func New(client *httpx.Client, limiter *ratelimit.Limiter, cache *cache.Cache, clientID string) *Provider {
	return &Provider{client: client, limiter: limiter, cache: cache, clientID: clientID}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves audio.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindAudio }

// Search queries the Jamendo track search endpoint for opts.Query, honouring opts.Cursor as a zero-based
// result offset and opts.Limit as the page size.
func (p *Provider) Search(ctx context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("jamendo: rate limit wait: %w", err)
	}

	offset := defaultOffset
	if n, err := strconv.Atoi(opts.Cursor); err == nil && n > 0 {
		offset = n
	}
	limit := assetcore.ClampLimit(opts.Limit)

	q := url.Values{
		"client_id":   {p.clientID},
		"format":      {"json"},
		"search":      {opts.Query},
		"limit":       {strconv.Itoa(limit)},
		"offset":      {strconv.Itoa(offset)},
		"audioformat": {dlFormatMP3},
	}
	reqURL := baseURL + "/tracks/?" + q.Encode()

	var env tracksEnvelope
	if err := p.client.GetJSON(ctx, reqURL, &env); err != nil {
		return assetcore.SearchResult{}, fmt.Errorf("jamendo: search: %w", err)
	}

	assets := make([]assetcore.Asset, 0, len(env.Results))
	for _, t := range env.Results {
		assets = append(assets, asset(t))
	}

	var nextCursor string
	if len(env.Results) == limit {
		nextCursor = strconv.Itoa(offset + limit)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

// resolveFormat maps an AudioFetchOpts.Format to the Jamendo audiodlformat selector, the Blob's
// Content-Type, and its file extension. Any value other than "ogg" (including "" and "mp3") resolves to
// the mp3 default.
func resolveFormat(format string) (sel, contentType, ext string) {
	if format == dlFormatOGG {
		return dlFormatOGG, "audio/ogg", ".ogg"
	}
	return dlFormatMP3, "audio/mpeg", ".mp3"
}

// metaCacheKey returns the on-disk cache key for id's detail metadata, namespaced by provider so it
// never collides with an audio bytes cache entry.
func metaCacheKey(id string) string { return cache.Key(providerName, "meta", id) }

// audioCacheKey returns the on-disk cache key for id's sel-encoded audio bytes, namespaced by provider
// and format selector so an mp3 and an ogg download for the same track never collide.
func audioCacheKey(sel, id string) string { return cache.Key(providerName, "audio", sel, id) }

// Fetch returns the audio clip identified by the provider-local id, encoded per opts.Format, checking
// the on-disk cache for both the sel-encoded audio bytes and the detail metadata before making any
// network call. On a hit for both entries, the Blob is rebuilt from cache alone, with zero HTTP
// requests. On a miss, it performs the detail lookup, downloads the audio bytes, and caches both before
// returning.
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

// cachedBlob rebuilds the Blob for id from the on-disk cache when both its sel-encoded audio bytes and
// detail metadata are present, returning hit=false with a zero Blob when either entry is missing.
func (p *Provider) cachedBlob(id, audioKey, metaKey, contentType, ext string) (assetcore.Blob, bool, error) {
	audioData, audioHit, err := p.cache.Get(audioKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("jamendo: cache get %s: %w", audioKey, err)
	}
	metaData, metaHit, err := p.cache.Get(metaKey)
	if err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("jamendo: cache get %s: %w", metaKey, err)
	}
	if !audioHit || !metaHit {
		return assetcore.Blob{}, false, nil
	}

	var t track
	if err := json.Unmarshal(metaData, &t); err != nil {
		return assetcore.Blob{}, false, fmt.Errorf("jamendo: decode cached metadata %s: %w", metaKey, err)
	}

	return blobFor(id, t, audioData, contentType, ext), true, nil
}

// fetchAndCache performs the by-id detail lookup and the audio download for id in the sel format,
// caching both the metadata and the audio bytes before returning the Blob. Jamendo returns HTTP 200 with
// an empty results array for an unknown id rather than a 404, so a not-found id is detected there; a
// track whose audiodownload is empty (download not allowed) is reported the same way.
func (p *Provider) fetchAndCache(
	ctx context.Context, id, sel, audioKey, metaKey, contentType, ext string,
) (assetcore.Blob, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: rate limit wait: %w", err)
	}

	q := url.Values{
		"client_id":     {p.clientID},
		"format":        {"json"},
		"id":            {id},
		"audiodlformat": {sel},
	}
	detailURL := baseURL + "/tracks/?" + q.Encode()

	var env tracksEnvelope
	if err := p.client.GetJSON(ctx, detailURL, &env); err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: fetch detail: %w", err)
	}
	if len(env.Results) == 0 {
		return assetcore.Blob{}, fmt.Errorf("jamendo: track %q: %w", id, assetcore.ErrNotFound)
	}
	t := env.Results[0]
	if t.AudioDownload == "" {
		return assetcore.Blob{}, fmt.Errorf("jamendo: track %q: %w", id, assetcore.ErrNotFound)
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: rate limit wait: %w", err)
	}
	audioData, err := p.client.GetBytes(ctx, t.AudioDownload)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: download audio: %w", err)
	}

	metaData, err := json.Marshal(t)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: encode metadata for %s: %w", metaKey, err)
	}
	if err := p.cache.Put(metaKey, metaData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: cache put %s: %w", metaKey, err)
	}
	if err := p.cache.Put(audioKey, audioData); err != nil {
		return assetcore.Blob{}, fmt.Errorf("jamendo: cache put %s: %w", audioKey, err)
	}

	return blobFor(id, t, audioData, contentType, ext), nil
}

// blobFor builds the assetcore.Blob for a fetched (or cache-rebuilt) track.
func blobFor(id string, t track, content []byte, contentType, ext string) assetcore.Blob {
	return assetcore.Blob{
		Asset:       asset(t),
		Content:     content,
		Filename:    id + ext,
		ContentType: contentType,
	}
}

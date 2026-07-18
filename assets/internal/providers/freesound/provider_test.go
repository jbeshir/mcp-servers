package freesound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const testAPIKey = "test-api-key"

// counters tracks the HTTP calls and headers seen by a testServer double, guarded for use across the
// server's request-handling goroutines and the test goroutine.
type counters struct {
	mu sync.Mutex

	previewRequests int32
	totalRequests   int32
	searchAuth      string
	detailAuth      string
	previewAuth     string
}

func (c *counters) recordSearchAuth(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.searchAuth = v
}

func (c *counters) SearchAuth() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.searchAuth
}

func (c *counters) recordDetailAuth(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.detailAuth = v
}

func (c *counters) DetailAuth() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.detailAuth
}

func (c *counters) recordPreviewAuth(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.previewAuth = v
}

func (c *counters) PreviewAuth() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previewAuth
}

// fixtureFields returns the name, username and license deed URL for one of the fixed test sound ids.
func fixtureFields(id string) (name, username, licenseURL string) {
	switch id {
	case "1001":
		return "Sound One", "User One", "http://creativecommons.org/licenses/by/4.0/"
	case "1002":
		return "Sound Two", "User Two", "https://creativecommons.org/licenses/by/4.0/"
	case "2000":
		return "Zero Sound", "User Three", "http://creativecommons.org/publicdomain/zero/1.0/"
	default:
		return "", "", ""
	}
}

// soundFixture builds the canned sound record for id, pointing its preview URLs back at the test server
// identified by host.
func soundFixture(host, id string) sound {
	name, username, licenseURL := fixtureFields(id)
	n, err := strconv.Atoi(id)
	if err != nil {
		panic(err)
	}
	return sound{
		ID: n, Name: name, Username: username, License: licenseURL,
		Previews: previews{
			PreviewHQMP3: "http://" + host + "/preview/" + id + "-hq.mp3",
			PreviewHQOGG: "http://" + host + "/preview/" + id + "-hq.ogg",
			PreviewLQMP3: "http://" + host + "/preview/" + id + "-lq.mp3",
		},
	}
}

// writeSearchResponse writes the canned search page for the given page: "" and "1" are a full page of
// two sounds with a next page, "2" is a final page of one sound with no next page.
func writeSearchResponse(w http.ResponseWriter, r *http.Request, page string) {
	var ids []string
	next := ""
	switch page {
	case "", "1":
		ids = []string{"1001", "1002"}
		next = "http://" + r.Host + "/search/text/?page=2"
	case "2":
		ids = []string{"2000"}
	}

	results := make([]sound, 0, len(ids))
	for _, id := range ids {
		results = append(results, soundFixture(r.Host, id))
	}

	env := searchEnvelope{Count: len(results), Next: next, Results: results}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		panic(err)
	}
}

// writeDetailResponse writes the canned by-id detail lookup for id, or a 404 for an unrecognized id
// (Freesound's own not-found status).
func writeDetailResponse(w http.ResponseWriter, r *http.Request, id string) {
	switch id {
	case "1001", "1002", "2000":
		if err := json.NewEncoder(w).Encode(soundFixture(r.Host, id)); err != nil {
			panic(err)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// testServer stands up a canned Freesound API double serving the search endpoint, the by-id detail
// endpoint, and a raw preview byte endpoint. baseURL is pointed at the server for the duration of the
// test.
func testServer(t *testing.T) *counters {
	t.Helper()

	c := &counters{}

	mux := http.NewServeMux()
	mux.HandleFunc("/search/text/", func(w http.ResponseWriter, r *http.Request) {
		c.recordSearchAuth(r.Header.Get("Authorization"))
		writeSearchResponse(w, r, r.URL.Query().Get("page"))
	})
	mux.HandleFunc("/sounds/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/sounds/") : len(r.URL.Path)-1]
		c.recordDetailAuth(r.Header.Get("Authorization"))
		writeDetailResponse(w, r, id)
	})
	mux.HandleFunc("/preview/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.previewRequests, 1)
		c.recordPreviewAuth(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake-audio-bytes"))
	})

	countingMux := http.NewServeMux()
	countingMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.totalRequests, 1)
		mux.ServeHTTP(w, r)
	})

	ts := httptest.NewServer(countingMux)
	t.Cleanup(ts.Close)

	orig := baseURL
	baseURL = ts.URL
	t.Cleanup(func() { baseURL = orig })

	return c
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c, testAPIKey)
}

func TestSearchMapsAssetsAndLicenses(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	a := res.Assets[0]
	require.Equal(t, "freesound:1001", a.ID)
	require.Equal(t, assetcore.KindAudio, a.Kind)
	require.Equal(t, "Sound One", a.Title)
	require.Equal(t, "User One", a.Source)
	require.Equal(t, "https://freesound.org/s/1001/", a.LandingURL)
	require.Contains(t, a.PreviewURL, "/preview/1001-lq.mp3")
	require.Equal(t, "CC-BY-4.0", a.License.SPDX)
	require.Equal(t, "CC BY 4.0", a.License.Name)
	require.Equal(t, "User One — Sound One (via Freesound)", a.License.Attribution)
	require.True(t, a.License.RequiresAttribution)
}

func TestSearchCC0SoundRequiresNoAttribution(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2, Cursor: "2"})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)

	lic := res.Assets[0].License
	require.Equal(t, cc0SPDX, lic.SPDX)
	require.False(t, lic.RequiresAttribution)
	require.Empty(t, lic.Attribution)
}

func TestSearchLicenseURLNormalization(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	// 1002's fixture license URL is the https + trailing-slash variant of the same by/4.0 deed as 1001.
	require.Equal(t, "CC-BY-4.0", res.Assets[1].License.SPDX)
	require.True(t, res.Assets[1].License.RequiresAttribution)
}

func TestSearchSendsTokenAuthHeader(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, "Token "+testAPIKey, c.SearchAuth())
}

func TestSearchPaginationAdvancesThenStops(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	page1, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, "2", page1.NextCursor)

	page2, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Empty(t, page2.NextCursor)
	require.Len(t, page2.Assets, 1)
}

func TestFetchColdDownloadsMP3ByDefault(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{})
	require.NoError(t, err)

	require.EqualValues(t, 1, atomic.LoadInt32(&c.previewRequests))
	require.Equal(t, "1001.mp3", blob.Filename)
	require.Equal(t, "audio/mpeg", blob.ContentType)
	require.Equal(t, []byte("fake-audio-bytes"), blob.Content)
	require.Equal(t, "Token "+testAPIKey, c.DetailAuth())
	require.Equal(t, "Token "+testAPIKey, c.PreviewAuth())
}

func TestFetchOggUsesDistinctCacheEntry(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&c.previewRequests))

	blob, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{Format: formatOGG})
	require.NoError(t, err)

	require.EqualValues(t, 2, atomic.LoadInt32(&c.previewRequests))
	require.Equal(t, "1001.ogg", blob.Filename)
	require.Equal(t, "audio/ogg", blob.ContentType)
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (audio bytes and detail
// metadata) are populated, a subsequent Fetch for the same id and format makes zero additional HTTP
// requests of any kind (detail lookup or preview download) against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "1002", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	warmTotal := atomic.LoadInt32(&c.totalRequests)
	warmPreview := atomic.LoadInt32(&c.previewRequests)
	require.Positive(t, warmTotal)

	blob2, err := p.Fetch(ctx, "1002", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-audio-bytes"), blob2.Content)
	require.Equal(t, warmTotal, atomic.LoadInt32(&c.totalRequests))
	require.Equal(t, warmPreview, atomic.LoadInt32(&c.previewRequests))
}

func TestFetchUnknownIDNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "unknown", assetcore.AudioFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindAudio, p.Kind())
}

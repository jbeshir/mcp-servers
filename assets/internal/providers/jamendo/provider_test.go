package jamendo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const testClientID = "test-client-id"

// counters tracks the HTTP calls made against a testServer double, guarded for use across the server's
// request-handling goroutines and the test goroutine.
type counters struct {
	mu sync.Mutex

	dlRequests     int32
	totalRequests  int32
	searchClientID string
	lastDLFormat   string
}

func (c *counters) recordSearchClientID(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.searchClientID = v
}

func (c *counters) SearchClientID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.searchClientID
}

func (c *counters) recordDLFormat(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastDLFormat = v
}

func (c *counters) LastDLFormat() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastDLFormat
}

// fixtureFields returns the name, artist and license deed URL for one of the fixed test track ids.
func fixtureFields(id string) (name, artist, licenseURL string) {
	switch id {
	case "1001":
		return "Song One", "Artist One", "https://creativecommons.org/licenses/by/3.0/"
	case "1002":
		return "Song Two", "Artist Two", "https://creativecommons.org/licenses/by-nc-sa/3.0/"
	case "2000":
		return "No Download", "Artist Three", "https://creativecommons.org/licenses/by/3.0/"
	case "3000":
		return "Legacy Track", "Artist Four", "https://creativecommons.org/licenses/sampling+/1.0/"
	default:
		return "", "", ""
	}
}

// trackFixture builds the canned track record for id, pointing its Audio/AudioDownload URLs back at the
// test server identified by host.
func trackFixture(host, id string) track {
	name, artist, licenseURL := fixtureFields(id)
	return track{
		ID: id, Name: name, ArtistName: artist,
		Audio: "http://" + host + "/stream/" + id, AudioDownload: "http://" + host + "/dl/" + id,
		LicenseCCURL: licenseURL, ShareURL: "https://www.jamendo.com/track/" + id,
	}
}

// writeSearchResponse writes the canned search page for the given offset: "" and "0" are a full page of
// two tracks, "2" is a short final page of one, anything else an empty page.
func writeSearchResponse(w http.ResponseWriter, r *http.Request, offset string) {
	var ids []string
	switch offset {
	case "", "0":
		ids = []string{"1001", "1002"}
	case "2":
		ids = []string{"3000"}
	}

	results := make([]track, 0, len(ids))
	for _, id := range ids {
		results = append(results, trackFixture(r.Host, id))
	}

	env := tracksEnvelope{Results: results}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		panic(err)
	}
}

// writeDetailResponse writes the canned by-id detail lookup for id: "2000" carries no audiodownload, an
// unrecognized id returns an empty results array (Jamendo's own not-found shape).
func writeDetailResponse(w http.ResponseWriter, r *http.Request, id string) {
	var results []track
	switch id {
	case "1001", "1002", "3000":
		results = []track{trackFixture(r.Host, id)}
	case "2000":
		nt := trackFixture(r.Host, id)
		nt.AudioDownload = ""
		results = []track{nt}
	}

	env := tracksEnvelope{Results: results}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		panic(err)
	}
}

// testServer stands up a canned Jamendo API double serving both the search and by-id detail shapes of
// GET /tracks/ from a single handler (routed on the presence of an "id" query param), plus a raw audio
// byte endpoint. baseURL is pointed at the server for the duration of the test.
func testServer(t *testing.T) *counters {
	t.Helper()

	c := &counters{}

	mux := http.NewServeMux()
	mux.HandleFunc("/tracks/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		id := q.Get("id")
		if id == "" {
			c.recordSearchClientID(q.Get("client_id"))
			writeSearchResponse(w, r, q.Get("offset"))
			return
		}

		c.recordDLFormat(q.Get("audiodlformat"))
		writeDetailResponse(w, r, id)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&c.dlRequests, 1)
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
	return New(client, limiter, c, testClientID)
}

func TestSearchMapsAssetsAndLicenses(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	a := res.Assets[0]
	require.Equal(t, "jamendo:1001", a.ID)
	require.Equal(t, assetcore.KindAudio, a.Kind)
	require.Equal(t, "Song One", a.Title)
	require.Equal(t, "Artist One", a.Source)
	require.Equal(t, "https://www.jamendo.com/track/1001", a.LandingURL)
	require.Contains(t, a.PreviewURL, "/stream/1001")
	require.Equal(t, "CC-BY-3.0", a.License.SPDX)
	require.Equal(t, "CC BY 3.0", a.License.Name)
	require.Equal(t, "Artist One — Song One (via Jamendo)", a.License.Attribution)
	require.True(t, a.License.RequiresAttribution)

	b := res.Assets[1]
	require.Equal(t, "jamendo:1002", b.ID)
	require.Equal(t, "Artist Two", b.Source)
	require.Equal(t, "https://www.jamendo.com/track/1002", b.LandingURL)
	require.Contains(t, b.PreviewURL, "/stream/1002")
	require.Equal(t, "CC-BY-NC-SA-3.0", b.License.SPDX)
	require.Equal(t, "CC BY NC SA 3.0", b.License.Name)
	require.Equal(t, "Artist Two — Song Two (via Jamendo)", b.License.Attribution)
	require.True(t, b.License.RequiresAttribution)
}

func TestSearchSendsClientIDAsQueryParam(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, testClientID, c.SearchClientID())
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

func TestSearchLegacyLicenseHasNoSPDXButRequiresAttribution(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "test", Limit: 2, Cursor: "2"})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)

	lic := res.Assets[0].License
	require.Empty(t, lic.SPDX)
	require.True(t, lic.RequiresAttribution)
}

func TestFetchColdDownloadsMP3ByDefault(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{})
	require.NoError(t, err)

	require.Equal(t, "mp32", c.LastDLFormat())
	require.EqualValues(t, 1, atomic.LoadInt32(&c.dlRequests))
	require.Equal(t, "1001.mp3", blob.Filename)
	require.Equal(t, "audio/mpeg", blob.ContentType)
	require.Equal(t, []byte("fake-audio-bytes"), blob.Content)
}

func TestFetchOggUsesDistinctCacheEntry(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&c.dlRequests))

	blob, err := p.Fetch(ctx, "1001", assetcore.AudioFetchOpts{Format: "ogg"})
	require.NoError(t, err)

	require.Equal(t, "ogg", c.LastDLFormat())
	require.EqualValues(t, 2, atomic.LoadInt32(&c.dlRequests))
	require.Equal(t, "1001.ogg", blob.Filename)
	require.Equal(t, "audio/ogg", blob.ContentType)
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (audio bytes and detail
// metadata) are populated, a subsequent Fetch for the same id and format makes zero additional HTTP
// requests of any kind (detail lookup or audio download) against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "1002", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	warmTotal := atomic.LoadInt32(&c.totalRequests)
	warmDL := atomic.LoadInt32(&c.dlRequests)
	require.Positive(t, warmTotal)

	blob2, err := p.Fetch(ctx, "1002", assetcore.AudioFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-audio-bytes"), blob2.Content)
	require.Equal(t, warmTotal, atomic.LoadInt32(&c.totalRequests))
	require.Equal(t, warmDL, atomic.LoadInt32(&c.dlRequests))
}

func TestFetchUnknownIDNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "unknown", assetcore.AudioFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestFetchDownloadNotAllowedNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "2000", assetcore.AudioFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindAudio, p.Kind())
}

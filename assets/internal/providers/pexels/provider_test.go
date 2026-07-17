package pexels

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

const (
	knownID          = 11111
	testKey          = "test-api-key"
	photographerName = "Someone"
)

// counters tracks the HTTP calls made against a testServer double, guarded for use across the server's
// request-handling goroutines and the test goroutine.
type counters struct {
	mu sync.Mutex

	imgRequests   int32
	totalRequests int32
	searchAuth    string
	detailAuth    string
	lastPerPage   string
}

func (c *counters) recordSearchAuth(h string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.searchAuth = h
}

func (c *counters) recordPerPage(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPerPage = v
}

func (c *counters) LastPerPage() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastPerPage
}

func (c *counters) recordDetailAuth(h string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.detailAuth = h
}

func (c *counters) SearchAuth() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.searchAuth
}

func (c *counters) DetailAuth() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.detailAuth
}

// testServer stands up a canned Pexels API double: a photo search list, a single-photo detail (whose
// src.original points back at the same test server's /img.jpg), and a raw image byte endpoint.
// baseURL is pointed at the server for the duration of the test.
func testServer(t *testing.T) *counters {
	t.Helper()

	c := &counters{}

	byPhoto := photo{
		ID: knownID, URL: "https://www.pexels.com/photo/" + strconv.Itoa(knownID),
		Photographer: photographerName, Alt: "A Photo",
		Src: photoSrc{Tiny: "https://example.com/tiny.jpg"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		c.recordSearchAuth(r.Header.Get("Authorization"))

		page := r.URL.Query().Get("page")
		c.recordPerPage(r.URL.Query().Get("per_page"))
		var resp searchResult
		switch page {
		case "", "1":
			resp = searchResult{
				TotalResults: 3, Page: 1, PerPage: 2,
				Photos:   []photo{byPhoto, withID(byPhoto, 22222)},
				NextPage: baseURL + "/v1/search?page=2",
			}
		case "2":
			resp = searchResult{
				TotalResults: 3, Page: 2, PerPage: 2,
				Photos: []photo{withID(byPhoto, 33333)},
			}
		default:
			t.Fatalf("unexpected page %q", page)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/photos/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/v1/photos/"):]
		if id != strconv.Itoa(knownID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		c.recordDetailAuth(r.Header.Get("Authorization"))

		with := byPhoto
		with.Src = photoSrc{Original: "http://" + r.Host + "/img.jpg", Tiny: "https://example.com/tiny.jpg"}
		_ = json.NewEncoder(w).Encode(with)
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&c.imgRequests, 1)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
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

// withID returns a copy of p with its ID (and derived landing URL) replaced.
func withID(p photo, id int) photo {
	p.ID = id
	p.URL = "https://www.pexels.com/photo/" + strconv.Itoa(id)
	return p
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c, testKey)
}

func TestSearchMapsAssetsAndAttribution(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	a := res.Assets[0]
	require.Equal(t, "pexels:"+strconv.Itoa(knownID), a.ID)
	require.Equal(t, assetcore.KindPhoto, a.Kind)
	require.Equal(t, "A Photo", a.Title)
	require.Equal(t, "https://www.pexels.com/photo/"+strconv.Itoa(knownID), a.LandingURL)
	require.Equal(t, "https://example.com/tiny.jpg", a.PreviewURL)
	require.Empty(t, a.License.SPDX)
	require.Equal(t, "Pexels License", a.License.Name)
	require.Equal(t, "https://www.pexels.com/license/", a.License.URL)
	require.Equal(t, "Photo by Someone on Pexels", a.License.Attribution)
	require.True(t, a.License.RequiresAttribution)

	require.Equal(t, testKey, c.SearchAuth())
}

func TestSearchTitleFallsBackToPhotographerCredit(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2, Cursor: "2"})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)
	require.Equal(t, "A Photo", res.Assets[0].Title)
}

func TestSearchPaginationAdvancesThenStops(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	page1, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, "2", page1.NextCursor)

	page2, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Empty(t, page2.NextCursor)
	require.Len(t, page2.Assets, 1)
}

func TestSearchCapsPerPageAtEighty(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 500})
	require.NoError(t, err)
	require.NotEmpty(t, res.Assets)
	require.Equal(t, "80", c.LastPerPage())
}

func TestFetchColdDownloadsOriginal(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, strconv.Itoa(knownID), assetcore.PhotoFetchOpts{})
	require.NoError(t, err)

	require.EqualValues(t, 1, atomic.LoadInt32(&c.imgRequests))
	require.Equal(t, strconv.Itoa(knownID)+".jpg", blob.Filename)
	require.Equal(t, "image/jpeg", blob.ContentType)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob.Content)
	require.Equal(t, "Photo by Someone on Pexels", blob.Asset.License.Attribution)
	require.True(t, blob.Asset.License.RequiresAttribution)

	require.Equal(t, testKey, c.DetailAuth())
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (image bytes and detail
// metadata) are populated, a subsequent Fetch for the same id makes zero additional HTTP requests of
// any kind (detail GET or image download) against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, strconv.Itoa(knownID), assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	warmTotal := atomic.LoadInt32(&c.totalRequests)
	warmImg := atomic.LoadInt32(&c.imgRequests)
	require.Positive(t, warmTotal)

	blob2, err := p.Fetch(ctx, strconv.Itoa(knownID), assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob2.Content)
	require.Equal(t, warmTotal, atomic.LoadInt32(&c.totalRequests))
	require.Equal(t, warmImg, atomic.LoadInt32(&c.imgRequests))
}

func TestFetchNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "unknown", assetcore.PhotoFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindPhoto, p.Kind())
}

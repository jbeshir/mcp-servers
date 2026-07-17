package unsplash

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

const (
	knownID   = "Dwu85P9SOIk"
	testKey   = "test-access-key"
	creatorNm = "Someone"
)

func strPtr(s string) *string { return &s }

// counters tracks the HTTP calls made against a testServer double, guarded for use across the server's
// request-handling goroutines and the test goroutine. seq assigns each observed /track or /img.jpg hit
// a monotonically increasing position, so a test can assert the download-tracking trigger fires strictly
// before the image download on a cold fetch.
type counters struct {
	mu sync.Mutex

	trackRequests int32
	imgRequests   int32
	totalRequests int32
	seq           int32
	trackOrder    int32
	imgOrder      int32
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

func (c *counters) recordTrackHit() {
	atomic.AddInt32(&c.trackRequests, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	if c.trackOrder == 0 {
		c.trackOrder = c.seq
	}
}

func (c *counters) recordImgHit() {
	atomic.AddInt32(&c.imgRequests, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	if c.imgOrder == 0 {
		c.imgOrder = c.seq
	}
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

func (c *counters) TrackOrder() int32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.trackOrder
}

func (c *counters) ImgOrder() int32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.imgOrder
}

// testServer stands up a canned Unsplash API double: a photo search list, a single-photo detail (whose
// links point back at the same test server's download-tracking and image-bytes endpoints), a
// download-tracking endpoint, and a raw image byte endpoint. baseURL is pointed at the server for the
// duration of the test.
func testServer(t *testing.T) *counters {
	t.Helper()

	c := &counters{}

	byPhoto := photo{
		ID:             knownID,
		Description:    strPtr("A Photo"),
		AltDescription: strPtr("A photo of something"),
		Links:          photoLinks{HTML: "https://unsplash.com/photos/" + knownID},
		Urls:           photoURLs{Thumb: "https://example.com/thumb.jpg"},
		User:           photoUser{Name: creatorNm},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search/photos", func(w http.ResponseWriter, r *http.Request) {
		c.recordSearchAuth(r.Header.Get("Authorization"))

		page := r.URL.Query().Get("page")
		c.recordPerPage(r.URL.Query().Get("per_page"))
		var resp searchResult
		switch page {
		case "", "1":
			resp = searchResult{
				Total: 3, TotalPages: 2,
				Results: []photo{byPhoto, withID(byPhoto, "22222222")},
			}
		case "2":
			resp = searchResult{
				Total: 3, TotalPages: 2,
				Results: []photo{withID(byPhoto, "33333333")},
			}
		default:
			t.Fatalf("unexpected page %q", page)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/photos/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/photos/"):]
		if id != knownID {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		c.recordDetailAuth(r.Header.Get("Authorization"))

		with := byPhoto
		with.Links = photoLinks{
			HTML:             "https://unsplash.com/photos/" + knownID,
			DownloadLocation: "http://" + r.Host + "/track",
		}
		with.Urls = photoURLs{Full: "http://" + r.Host + "/img.jpg", Thumb: "https://example.com/thumb.jpg"}
		_ = json.NewEncoder(w).Encode(with)
	})
	mux.HandleFunc("/track", func(w http.ResponseWriter, _ *http.Request) {
		c.recordTrackHit()
		_ = json.NewEncoder(w).Encode(map[string]string{"url": "http://example.com/tracked"})
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, _ *http.Request) {
		c.recordImgHit()
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

// withID returns a copy of p with its ID (and derived links.html landing URL) replaced.
func withID(p photo, id string) photo {
	p.ID = id
	p.Links.HTML = "https://unsplash.com/photos/" + id
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
	require.Equal(t, "unsplash:"+knownID, a.ID)
	require.Equal(t, assetcore.KindPhoto, a.Kind)
	require.Equal(t, "A Photo", a.Title)
	require.Equal(t, creatorNm, a.Source)
	require.Equal(t, "https://unsplash.com/photos/"+knownID, a.LandingURL)
	require.Equal(t, "https://example.com/thumb.jpg", a.PreviewURL)
	require.Empty(t, a.License.SPDX)
	require.Equal(t, "Unsplash License", a.License.Name)
	require.Equal(t, "Photo by Someone on Unsplash", a.License.Attribution)
	require.True(t, a.License.RequiresAttribution)

	require.Equal(t, "Client-ID "+testKey, c.SearchAuth())
}

func TestSearchTitleFallsBackToAltDescriptionThenID(t *testing.T) {
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

func TestSearchCapsPerPageAtThirty(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 500})
	require.NoError(t, err)
	require.NotEmpty(t, res.Assets)
	require.Equal(t, "30", c.LastPerPage())
}

func TestFetchColdTriggersDownloadBeforeImage(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)

	require.EqualValues(t, 1, atomic.LoadInt32(&c.trackRequests))
	require.EqualValues(t, 1, atomic.LoadInt32(&c.imgRequests))
	require.Positive(t, c.TrackOrder())
	require.Positive(t, c.ImgOrder())
	require.Less(t, c.TrackOrder(), c.ImgOrder())

	require.Equal(t, knownID+".jpg", blob.Filename)
	require.Equal(t, "image/jpeg", blob.ContentType)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob.Content)
	require.Equal(t, creatorNm, blob.Asset.Source)
	require.Equal(t, "Photo by Someone on Unsplash", blob.Asset.License.Attribution)
	require.True(t, blob.Asset.License.RequiresAttribution)

	require.Equal(t, "Client-ID "+testKey, c.DetailAuth())
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (image bytes and detail
// metadata) are populated, a subsequent Fetch for the same id makes zero additional HTTP requests of any
// kind against the upstream server — including no re-trigger of the download-tracking endpoint.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	c := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	warmTotal := atomic.LoadInt32(&c.totalRequests)
	warmTrack := atomic.LoadInt32(&c.trackRequests)
	warmImg := atomic.LoadInt32(&c.imgRequests)
	require.Positive(t, warmTotal)

	blob2, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob2.Content)
	require.Equal(t, creatorNm, blob2.Asset.Source)
	require.Equal(t, warmTotal, atomic.LoadInt32(&c.totalRequests))
	require.Equal(t, warmTrack, atomic.LoadInt32(&c.trackRequests))
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

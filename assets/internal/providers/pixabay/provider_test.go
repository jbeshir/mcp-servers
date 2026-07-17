package pixabay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const (
	knownID   = "12345"
	testKey   = "test-api-key"
	unknownID = "99999"
)

// testServer stands up a canned Pixabay API double: a single "/api/" endpoint that serves both the
// paged search query (q= present) and the single-hit id lookup Fetch uses (id= present, no detail
// endpoint on the real API), plus a raw image byte endpoint. imgRequests counts how many times the raw
// image endpoint was hit, and totalRequests counts every request the server receives (search/lookup and
// image alike), for cache assertions. Every request is asserted to carry a non-empty key query param.
func testServer(t *testing.T) (imgRequests, totalRequests *int32) {
	t.Helper()

	imgRequests = new(int32)
	totalRequests = new(int32)

	byHit := hit{
		ID: 12345, PageURL: "https://pixabay.com/photos/cat-12345/", Tags: "cat, kitten, animal",
		PreviewURL:   "https://cdn.pixabay.com/photo/preview_12345.jpg",
		WebformatURL: "https://cdn.pixabay.com/photo/webformat_12345.jpg", User: "someuser",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch id := r.URL.Query().Get("id"); id {
		case "":
			// Search branch.
			page := r.URL.Query().Get("page")
			var resp searchResult
			switch page {
			case "", "1":
				resp = searchResult{
					Total: 3, TotalHits: 3,
					Hits: []hit{byHit, withID(byHit, 67890)},
				}
			case "2":
				resp = searchResult{
					Total: 3, TotalHits: 3,
					Hits: []hit{withID(byHit, 11111)},
				}
			default:
				t.Fatalf("unexpected page %q", page)
			}
			_ = json.NewEncoder(w).Encode(resp)
			return

		case knownID:
			with := byHit
			with.LargeImageURL = "http://" + r.Host + "/img.jpg"
			_ = json.NewEncoder(w).Encode(searchResult{Total: 1, TotalHits: 1, Hits: []hit{with}})
			return

		case unknownID:
			_ = json.NewEncoder(w).Encode(searchResult{Total: 0, TotalHits: 0, Hits: []hit{}})
			return

		default:
			t.Fatalf("unexpected id %q", id)
		}
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(imgRequests, 1)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	})

	countingMux := http.NewServeMux()
	countingMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(totalRequests, 1)
		mux.ServeHTTP(w, r)
	})

	ts := httptest.NewServer(countingMux)
	t.Cleanup(ts.Close)

	orig := baseURL
	baseURL = ts.URL
	t.Cleanup(func() { baseURL = orig })

	return imgRequests, totalRequests
}

// withID returns a copy of h with its ID replaced.
func withID(h hit, id int) hit {
	h.ID = id
	return h
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c, testKey)
}

func TestSearchMapsAssetsAndAttribution(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	a := res.Assets[0]
	require.Equal(t, "pixabay:12345", a.ID)
	require.Equal(t, assetcore.KindPhoto, a.Kind)
	require.Equal(t, "cat", a.Title)
	require.Equal(t, []string{"cat", "kitten", "animal"}, a.Tags)
	require.Equal(t, "https://pixabay.com/photos/cat-12345/", a.LandingURL)
	require.Equal(t, "https://cdn.pixabay.com/photo/preview_12345.jpg", a.PreviewURL)
	require.Empty(t, a.License.SPDX)
	require.Equal(t, "Pixabay Content License", a.License.Name)
	require.Equal(t, "https://pixabay.com/service/license-summary/", a.License.URL)
	require.Equal(t, "Image by someuser on Pixabay", a.License.Attribution)
	require.False(t, a.License.RequiresAttribution)
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
	require.Equal(t, "pixabay:11111", page2.Assets[0].ID)
}

func TestFetchDerivesContentTypeAndCaches(t *testing.T) {
	imgRequests, _ := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, knownID+".jpg", blob.Filename)
	require.Equal(t, "image/jpeg", blob.ContentType)
	require.Equal(t, "pixabay:"+knownID, blob.Asset.ID)
	require.False(t, blob.Asset.License.RequiresAttribution)
	require.NotEmpty(t, blob.Asset.License.Attribution)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob.Content)
	require.EqualValues(t, 1, atomic.LoadInt32(imgRequests))

	// A second fetch must hit the on-disk cache, not re-download the image.
	blob2, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, blob.Content, blob2.Content)
	require.EqualValues(t, 1, atomic.LoadInt32(imgRequests))
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (image bytes and detail
// metadata) are populated, a subsequent Fetch for the same id makes zero additional HTTP requests of
// any kind (lookup GET or image download) against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	_, totalRequests := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	warmCount := atomic.LoadInt32(totalRequests)
	require.Positive(t, warmCount)

	blob2, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-jpeg-bytes"), blob2.Content)
	require.Equal(t, warmCount, atomic.LoadInt32(totalRequests))
}

func TestFetchNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, unknownID, assetcore.PhotoFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindPhoto, p.Kind())
}

func TestTitleFallsBackToIDWhenTagsBlank(t *testing.T) {
	require.Equal(t, strconv.Itoa(42), title(hit{ID: 42, Tags: ""}))
	require.Equal(t, strconv.Itoa(42), title(hit{ID: 42, Tags: "   "}))
	require.Equal(t, "cat", title(hit{ID: 42, Tags: "cat, kitten"}))
}

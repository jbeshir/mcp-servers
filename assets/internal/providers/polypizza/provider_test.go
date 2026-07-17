package polypizza

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const (
	testAPIKey = "test-api-key"
	byID       = "5EGWBMpuXq"
	cc0ID      = "aabbccddee"
)

// testServer stands up a canned Poly Pizza API double: a keyword search list, a single-model detail
// endpoint, and a raw .glb byte endpoint. dlRequests counts how many times the raw .glb endpoint was
// hit, and totalRequests counts every request the server receives (detail + download alike), for cache
// assertions. The detail endpoint's Download field points back at the same test server's /dl.glb. Every
// request to /search/ or /model/ is asserted to carry the x-auth-token header; every request to
// /dl.glb is asserted NOT to.
func testServer(t *testing.T) (dlRequests, totalRequests *int32) {
	t.Helper()

	dlRequests = new(int32)
	totalRequests = new(int32)

	byModel := model{
		ID: byID, Title: "A Chair", Thumbnail: "https://example.com/chair-thumb.png",
		Creator: creator{Username: "Someone"}, Licence: "CC-BY 3.0",
		Attribution: "\"A Chair\" by Someone is licensed under CC-BY 3.0", TriCount: 1200,
	}
	cc0Model := model{
		ID: cc0ID, Title: "A Cube", Thumbnail: "https://example.com/cube-thumb.png",
		Creator: creator{Username: "Someone Else"}, Licence: "CC0 1.0", TriCount: 12,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Auth-Token"); got != testAPIKey {
			t.Errorf("search request X-Auth-Token = %q, want %q", got, testAPIKey)
		}

		page := r.URL.Query().Get("page")
		var env searchEnvelope
		switch page {
		case "", "1":
			env = searchEnvelope{Total: 3, Results: []model{byModel, cc0Model}}
		case "2":
			env = searchEnvelope{Total: 3, Results: []model{withID(byModel, "thirdModel")}}
		default:
			t.Fatalf("unexpected page %q", page)
		}
		_ = json.NewEncoder(w).Encode(env)
	})
	mux.HandleFunc("/model/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Auth-Token"); got != testAPIKey {
			t.Errorf("model request X-Auth-Token = %q, want %q", got, testAPIKey)
		}

		id := strings.TrimPrefix(r.URL.Path, "/model/")
		var m model
		switch id {
		case byID:
			m = byModel
		case cc0ID:
			m = cc0Model
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		m.Download = "http://" + r.Host + "/dl.glb"
		_ = json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("/dl.glb", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(dlRequests, 1)
		if got := r.Header.Get("X-Auth-Token"); got != "" {
			t.Errorf("dl.glb request unexpectedly carried X-Auth-Token %q", got)
		}
		_, _ = w.Write([]byte("fake-glb-bytes"))
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

	return dlRequests, totalRequests
}

// withID returns a copy of m with its ID replaced.
func withID(m model, id string) model {
	m.ID = id
	return m
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c, testAPIKey)
}

func TestSearchMapsAssetsAndLicense(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "furniture", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	ccBy := res.Assets[0]
	require.Equal(t, "polypizza:"+byID, ccBy.ID)
	require.Equal(t, assetcore.KindModel, ccBy.Kind)
	require.Equal(t, "A Chair", ccBy.Title)
	require.Equal(t, "Someone", ccBy.Source)
	require.Equal(t, "https://example.com/chair-thumb.png", ccBy.PreviewURL)
	require.Equal(t, "https://poly.pizza/m/"+byID, ccBy.LandingURL)
	require.Equal(t, "CC-BY-3.0", ccBy.License.SPDX)
	require.Equal(t, "CC-BY 3.0", ccBy.License.Name)
	require.Equal(t, "\"A Chair\" by Someone is licensed under CC-BY 3.0", ccBy.License.Attribution)
	require.True(t, ccBy.License.RequiresAttribution)

	cc0 := res.Assets[1]
	require.Equal(t, "polypizza:"+cc0ID, cc0.ID)
	require.Equal(t, "Someone Else", cc0.Source)
	require.Equal(t, "CC0-1.0", cc0.License.SPDX)
	require.False(t, cc0.License.RequiresAttribution)
	require.Empty(t, cc0.License.Attribution)
}

func TestLicenseFallsBackToCreatorAttribution(t *testing.T) {
	m := model{ID: "x", Creator: creator{Username: "Jane"}, Licence: "CC-BY-SA 4.0"}
	got := license(m)
	require.Equal(t, "CC-BY-SA-4.0", got.SPDX)
	require.True(t, got.RequiresAttribution)
	require.Equal(t, "Model by Jane on Poly Pizza", got.Attribution)
}

func TestSearchPaginationAdvancesThenStops(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	page1, err := p.Search(ctx, assetcore.SearchOpts{Query: "furniture", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, "2", page1.NextCursor)

	page2, err := p.Search(ctx, assetcore.SearchOpts{Query: "furniture", Limit: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Empty(t, page2.NextCursor)
	require.Len(t, page2.Assets, 1)
}

func TestFetchColdDownloadsGLB(t *testing.T) {
	dlRequests, _ := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, byID, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, byID+".glb", blob.Filename)
	require.Equal(t, "model/gltf-binary", blob.ContentType)
	require.Equal(t, "polypizza:"+byID, blob.Asset.ID)
	require.Equal(t, "Someone", blob.Asset.Source)
	require.Equal(t, "CC-BY-3.0", blob.Asset.License.SPDX)
	require.Equal(t, []byte("fake-glb-bytes"), blob.Content)
	require.EqualValues(t, 1, atomic.LoadInt32(dlRequests))

	// A second fetch must hit the on-disk cache, not re-download the .glb.
	blob2, err := p.Fetch(ctx, byID, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, blob.Content, blob2.Content)
	require.EqualValues(t, 1, atomic.LoadInt32(dlRequests))
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once both cache entries (glb bytes and detail
// metadata) are populated, a subsequent Fetch for the same id makes zero additional HTTP requests of
// any kind (detail GET or glb download) against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	_, totalRequests := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, byID, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	warmCount := atomic.LoadInt32(totalRequests)
	require.Positive(t, warmCount)

	blob2, err := p.Fetch(ctx, byID, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-glb-bytes"), blob2.Content)
	require.Equal(t, "CC-BY-3.0", blob2.Asset.License.SPDX)
	require.Equal(t, warmCount, atomic.LoadInt32(totalRequests))
}

func TestFetchNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "unknown", assetcore.ModelFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindModel, p.Kind())
}

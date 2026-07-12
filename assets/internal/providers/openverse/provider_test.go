package openverse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const knownID = "11111111-1111-1111-1111-111111111111"

// testServer stands up a canned Openverse API double: a search list, a single-image detail, and a raw
// image byte endpoint. imgRequests counts how many times the raw image endpoint was hit, and
// totalRequests counts every request the server receives (detail + image alike), for cache assertions.
// The detail endpoint's url field points back at the same test server's /img.jpg.
func testServer(t *testing.T) (imgRequests, totalRequests *int32) {
	t.Helper()

	imgRequests = new(int32)
	totalRequests = new(int32)

	byResult := imageResult{
		ID: knownID, Title: "A Photo", ForeignLandingURL: "https://example.com/photo",
		Thumbnail: "https://example.com/thumb.jpg", Creator: "Someone",
		License: "by", LicenseVersion: "4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
		Attribution: "\"A Photo\" by Someone is licensed under CC BY 4.0.", Source: "flickr",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/images/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch path {
		case "/v1/images/":
			page := r.URL.Query().Get("page")
			var resp searchResult
			switch page {
			case "", "1":
				resp = searchResult{
					ResultCount: 3, PageCount: 2, Page: 1, PageSize: 2,
					Results: []imageResult{byResult, withID(byResult, "22222222-2222-2222-2222-222222222222")},
				}
			case "2":
				resp = searchResult{
					ResultCount: 3, PageCount: 2, Page: 2, PageSize: 2,
					Results: []imageResult{withCode(byResult, "cc0", "1.0")},
				}
			default:
				t.Fatalf("unexpected page %q", page)
			}
			_ = json.NewEncoder(w).Encode(resp)

		case "/v1/images/" + knownID + "/":
			with := byResult
			with.URL = "http://" + r.Host + "/img.jpg"
			_ = json.NewEncoder(w).Encode(with)

		case "/v1/images/unknown/":
			w.WriteHeader(http.StatusNotFound)

		default:
			http.NotFound(w, r)
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

// withID returns a copy of r with its ID replaced.
func withID(r imageResult, id string) imageResult {
	r.ID = id
	return r
}

// withCode returns a copy of r with its license code and version replaced.
func withCode(r imageResult, code, version string) imageResult {
	r.License = code
	r.LicenseVersion = version
	return r
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c)
}

func TestLicenseMapping(t *testing.T) {
	tests := []struct {
		code, version   string
		wantSPDX        string
		wantName        string
		wantAttribution bool
	}{
		{"by", "4.0", "CC-BY-4.0", "CC BY 4.0", true},
		{"by-nc-sa", "4.0", "CC-BY-NC-SA-4.0", "CC BY NC SA 4.0", true},
		{"cc0", "1.0", "CC0-1.0", "CC0 1.0", false},
		{"pdm", "1.0", "", "Public Domain Mark 1.0", false},
	}
	for _, tc := range tests {
		require.Equal(t, tc.wantSPDX, spdxFor(tc.code, tc.version), "spdxFor(%q, %q)", tc.code, tc.version)
		require.Equal(t, tc.wantName, humanName(tc.code, tc.version), "humanName(%q, %q)", tc.code, tc.version)
		require.Equal(t, tc.wantAttribution, requiresAttribution(tc.code), "requiresAttribution(%q)", tc.code)
	}
}

func TestSearchMapsAssetsAndAttribution(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "cat", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	a := res.Assets[0]
	require.Equal(t, "openverse:"+knownID, a.ID)
	require.Equal(t, "flickr", a.Source)
	require.Equal(t, assetcore.KindPhoto, a.Kind)
	require.Equal(t, "A Photo", a.Title)
	require.Equal(t, "https://example.com/photo", a.LandingURL)
	require.Equal(t, "https://example.com/thumb.jpg", a.PreviewURL)
	require.Equal(t, "CC-BY-4.0", a.License.SPDX)
	require.Equal(t, "CC BY 4.0", a.License.Name)
	require.Equal(t, "\"A Photo\" by Someone is licensed under CC BY 4.0.", a.License.Attribution)
	require.True(t, a.License.RequiresAttribution)
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
	require.False(t, page2.Assets[0].License.RequiresAttribution)
	require.Equal(t, "CC0-1.0", page2.Assets[0].License.SPDX)
}

func TestFetchDerivesContentTypeAndCaches(t *testing.T) {
	imgRequests, _ := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, knownID, assetcore.PhotoFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, knownID+".jpg", blob.Filename)
	require.Equal(t, "image/jpeg", blob.ContentType)
	require.Equal(t, "openverse:"+knownID, blob.Asset.ID)
	require.Equal(t, "CC-BY-4.0", blob.Asset.License.SPDX)
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
// any kind (detail GET or image download) against the upstream server.
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
	require.Equal(t, "CC-BY-4.0", blob2.Asset.License.SPDX)
	require.Equal(t, warmCount, atomic.LoadInt32(totalRequests))
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

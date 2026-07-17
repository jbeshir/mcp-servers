package polyhaven

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
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

const knownSlug = "ArmChair_01"

// testServer stands up a canned Poly Haven API double: GET /assets?type=models serving a fixed
// catalogue keyed by slug, GET /files/{slug} serving a download manifest whose gltf.1k branch points
// back at this same server (an unknown slug gets a 404, mirroring the real API), and the individual file
// endpoints it references. fileRequests counts the main .gltf/texture/binary downloads; totalRequests
// counts every request the server receives, for warm-cache assertions.
func testServer(t *testing.T) (fileRequests, totalRequests *int32) {
	t.Helper()

	fileRequests = new(int32)
	totalRequests = new(int32)

	catalogue := map[string]assetMeta{
		knownSlug: {
			Name:         "Arm Chair 01",
			Categories:   []string{"furniture", "seating"},
			Tags:         []string{"chair", "wood"},
			Authors:      map[string]string{"Kirill Sannikov": "All"},
			ThumbnailURL: "https://cdn.example.com/armchair.png",
		},
		"WoodenBarrel_01": {
			Name:       "Wooden Barrel 01",
			Categories: []string{"props"},
			Tags:       []string{"barrel", "wood"},
		},
		"MetalBucket_02": {
			Name:       "Metal Bucket 02",
			Categories: []string{"props"},
			Tags:       []string{"bucket", "metal"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/assets", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(catalogue)
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/files/")
		if slug != knownSlug {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		base := "http://" + r.Host
		manifest := filesManifest{
			Gltf: map[string]map[string]map[string]gltfEntry{
				"1k": {
					"gltf": {
						knownSlug + "_1k.gltf": {
							URL: base + "/main.gltf",
							Include: map[string]fileRef{
								"textures/diff_1k.jpg": {URL: base + "/tex.jpg"},
								"Model.bin":            {URL: base + "/model.bin"},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/main.gltf", servesFile(fileRequests, "main gltf bytes"))
	mux.HandleFunc("/tex.jpg", servesFile(fileRequests, "texture bytes"))
	mux.HandleFunc("/model.bin", servesFile(fileRequests, "binary bytes"))

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

	return fileRequests, totalRequests
}

// servesFile returns a handler that writes body as the response and counts itself in requests.
func servesFile(requests *int32, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(requests, 1)
		_, _ = w.Write([]byte(body))
	}
}

// newTestProvider builds a Provider wired to a fresh cache dir and a permissive rate limiter.
func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	return New(httpx.New(httpx.Config{}), ratelimit.New(1000, 10), cache.New(t.TempDir()))
}

func TestSearchMapsAssets(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Query: "Arm"})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)

	a := res.Assets[0]
	require.Equal(t, "polyhaven:"+knownSlug, a.ID)
	require.Equal(t, assetcore.KindModel, a.Kind)
	require.Equal(t, "Arm Chair 01", a.Title)
	require.Equal(t, []string{"chair", "wood"}, a.Tags)
	require.Equal(t, "https://cdn.example.com/armchair.png", a.PreviewURL)
	require.Equal(t, "https://polyhaven.com/a/"+knownSlug, a.LandingURL)
	require.Equal(t, cc0License, a.License)
}

func TestSearchQueryMatchesNameOrSlugCaseInsensitively(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Query: "barrel"})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)
	require.Equal(t, "polyhaven:WoodenBarrel_01", res.Assets[0].ID)
}

func TestSearchEmptyQueryMatchesEverything(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Limit: 10})
	require.NoError(t, err)
	require.Len(t, res.Assets, 3)
	require.Empty(t, res.NextCursor)
}

func TestSearchPaginationAdvancesThenStops(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)

	page1, err := p.Search(t.Context(), assetcore.SearchOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page1.Assets, 2)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := p.Search(t.Context(), assetcore.SearchOpts{Limit: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Len(t, page2.Assets, 1)
	require.Empty(t, page2.NextCursor)
}

// TestFetchAssemblesZip proves a cold Fetch downloads the main .gltf plus every Include entry and packs
// them into a ZIP at their exact relative paths, defaulting to the "1k" resolution.
func TestFetchAssemblesZip(t *testing.T) {
	fileRequests, _ := testServer(t)
	p := newTestProvider(t)

	blob, err := p.Fetch(t.Context(), knownSlug, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, "application/zip", blob.ContentType)
	require.Equal(t, knownSlug+"_1k.zip", blob.Filename)
	require.Equal(t, "polyhaven:"+knownSlug, blob.Asset.ID)
	require.EqualValues(t, 3, atomic.LoadInt32(fileRequests))

	zr, err := zip.NewReader(bytes.NewReader(blob.Content), int64(len(blob.Content)))
	require.NoError(t, err)

	contents := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		contents[f.Name] = string(data)
	}

	require.Equal(t, "main gltf bytes", contents[knownSlug+"_1k.gltf"])
	require.Equal(t, "texture bytes", contents["textures/diff_1k.jpg"])
	require.Equal(t, "binary bytes", contents["Model.bin"])
}

// TestFetchWarmCacheMakesNoHTTPRequests proves that once a slug/resolution's ZIP is cached, a
// subsequent Fetch makes zero additional HTTP requests of any kind against the upstream server.
func TestFetchWarmCacheMakesNoHTTPRequests(t *testing.T) {
	_, totalRequests := testServer(t)
	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), knownSlug, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	warmCount := atomic.LoadInt32(totalRequests)
	require.Positive(t, warmCount)

	blob2, err := p.Fetch(t.Context(), knownSlug, assetcore.ModelFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, knownSlug+"_1k.zip", blob2.Filename)
	require.Equal(t, warmCount, atomic.LoadInt32(totalRequests))
}

func TestFetchUnknownSlugReturnsErrNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), "NotReal_01", assetcore.ModelFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindModel, p.Kind())
}

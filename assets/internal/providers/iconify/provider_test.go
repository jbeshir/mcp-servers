package iconify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// testServer stands up a canned Iconify API double and returns the count of SVG requests served plus
// the most recently requested SVG URL, for assertions on caching and query encoding.
func testServer(t *testing.T) (svgRequests *int32, lastSVGURL *string) {
	t.Helper()

	svgRequests = new(int32)
	lastSVGURL = new(string)

	mdiLicense := collectionInfo{
		Name:    "Material Design Icons",
		License: licenseInfo{Title: "Apache 2.0", SPDX: "Apache-2.0", URL: "https://apache.org/licenses/LICENSE-2.0"},
	}
	lucideLicense := collectionInfo{
		Name:    "Lucide",
		License: licenseInfo{Title: "ISC License", SPDX: "ISC", URL: "https://opensource.org/licenses/ISC"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/collections", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]collectionInfo{
			"mdi":    mdiLicense,
			"lucide": lucideLicense,
		})
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))

		var resp searchResponse
		switch start {
		case 0:
			resp = searchResponse{
				Icons:       []string{"lucide:home", "mdi:account"},
				Total:       4,
				Start:       0,
				Limit:       32,
				Collections: map[string]collectionInfo{"mdi": mdiLicense},
			}
		case 2:
			resp = searchResponse{
				Icons: []string{"lucide:settings", "mdi:bell"},
				Total: 4,
				Start: 2,
				Limit: 32,
			}
		default:
			t.Fatalf("unexpected search start %d", start)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".svg") {
			http.NotFound(w, r)
			return
		}

		atomic.AddInt32(svgRequests, 1)
		*lastSVGURL = r.URL.String()

		if strings.Contains(r.URL.Path, "unknown-icon") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	orig := baseURL
	baseURL = ts.URL
	t.Cleanup(func() { baseURL = orig })

	return svgRequests, lastSVGURL
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	client := httpx.New(httpx.Config{})
	limiter := ratelimit.New(1000, 10)
	c := cache.New(t.TempDir())
	return New(client, limiter, c)
}

func TestSearchMapsCompositeIDsAndLicenses(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{Query: "a", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res.Assets, 2)

	require.Equal(t, "iconify:lucide/home", res.Assets[0].ID)
	require.Equal(t, "lucide", res.Assets[0].Source)
	require.Equal(t, "home", res.Assets[0].Title)
	require.Equal(t, assetcore.KindIcon, res.Assets[0].Kind)
	// lucide is absent from the search response's own collections map, so its license must come from
	// the /collections fallback cache.
	require.Equal(t, "ISC", res.Assets[0].License.SPDX)

	// mdi is present inline in the search response's collections map.
	require.Equal(t, "iconify:mdi/account", res.Assets[1].ID)
	require.Equal(t, "Apache-2.0", res.Assets[1].License.SPDX)
}

func TestSearchPaginationAdvancesThenStops(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	page1, err := p.Search(ctx, assetcore.SearchOpts{Query: "a", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, "2", page1.NextCursor)

	page2, err := p.Search(ctx, assetcore.SearchOpts{Query: "a", Limit: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Empty(t, page2.NextCursor)
	require.Len(t, page2.Assets, 2)
}

func TestSearchHonoursSourcesFilter(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	res, err := p.Search(ctx, assetcore.SearchOpts{
		Query:   "a",
		Limit:   2,
		Sources: assetcore.Filter{Only: []string{"mdi"}},
	})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)
	require.Equal(t, "mdi", res.Assets[0].Source)
}

func TestFetchEncodesColorAndCachesResult(t *testing.T) {
	svgRequests, lastSVGURL := testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	blob, err := p.Fetch(ctx, "lucide/home", assetcore.IconFetchOpts{Color: "#ff0000", Size: 24})
	require.NoError(t, err)
	require.Equal(t, "home.svg", blob.Filename)
	require.Equal(t, "image/svg+xml", blob.ContentType)
	require.Equal(t, "iconify:lucide/home", blob.Asset.ID)
	require.Equal(t, "ISC", blob.Asset.License.SPDX)
	require.NotEmpty(t, blob.Content)

	require.Contains(t, *lastSVGURL, "color=%23ff0000")
	require.Contains(t, *lastSVGURL, "height=24")
	require.EqualValues(t, 1, atomic.LoadInt32(svgRequests))

	// A second fetch with identical parameters must hit the on-disk cache, not the network.
	blob2, err := p.Fetch(ctx, "lucide/home", assetcore.IconFetchOpts{Color: "#ff0000", Size: 24})
	require.NoError(t, err)
	require.Equal(t, blob.Content, blob2.Content)
	require.EqualValues(t, 1, atomic.LoadInt32(svgRequests))
}

func TestFetchNotFound(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "lucide/unknown-icon", assetcore.IconFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestFetchMalformedID(t *testing.T) {
	testServer(t)
	p := newTestProvider(t)
	ctx := context.Background()

	_, err := p.Fetch(ctx, "no-slash-here", assetcore.IconFetchOpts{})
	require.Error(t, err)
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestProviderIdentity(t *testing.T) {
	p := newTestProvider(t)
	require.Equal(t, providerName, p.Name())
	require.Equal(t, assetcore.KindIcon, p.Kind())
}

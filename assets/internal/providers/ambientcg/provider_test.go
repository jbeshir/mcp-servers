package ambientcg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const zipBytes = "canned zip bytes"

// newTestProvider builds a Provider wired to a fresh cache dir and a permissive rate limiter.
func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	return New(httpx.New(httpx.Config{}), ratelimit.New(1000, 10), cache.New(t.TempDir()))
}

// withAPIBase overrides apiBaseURL for the duration of the test.
func withAPIBase(t *testing.T, base string) {
	t.Helper()

	orig := apiBaseURL
	apiBaseURL = base
	t.Cleanup(func() { apiBaseURL = orig })
}

// stubAsset describes one material to serve from the test full_json endpoint.
type stubAsset struct {
	AssetID         string
	DisplayName     string
	DisplayCategory string
	Tags            []string
	Attributes      []string // "<Resolution>-<Format>" strings this asset offers
}

// newStubServer serves /api/v2/full_json plus /file.zip returning zipBytes, mirroring the real API's
// two distinct query modes: a single-asset lookup keyed by id= (exact assetId match, as Fetch uses) and
// a search keyed by q= (substring match against DisplayName, or all when q is empty). Crucially q= does
// NOT match against assetId — the real full_json search does not resolve an assetId token — so a Fetch
// that mistakenly looks an asset up by q= finds nothing. Each downloadLink for a stub's attributes
// points back at /file.zip on the same server; zipRequests counts /file.zip hits.
func newStubServer(t *testing.T, assets []stubAsset, zipRequests *int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/file.zip", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(zipRequests, 1)
		_, _ = w.Write([]byte(zipBytes))
	})
	mux.HandleFunc("/api/v2/full_json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		base := "http://" + r.Host

		var matched []stubAsset
		if id := q.Get("id"); id != "" {
			for _, a := range assets {
				if a.AssetID == id {
					matched = append(matched, a)
				}
			}
		} else {
			query := q.Get("q")
			for _, a := range assets {
				if query == "" || query == a.DisplayName {
					matched = append(matched, a)
				}
			}
		}

		env := searchEnvelope{NumberOfResults: len(matched)}
		for _, a := range page(matched, q.Get("offset"), q.Get("limit")) {
			env.FoundAssets = append(env.FoundAssets, toFoundAsset(a, base))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(env)
	})

	return httptest.NewServer(mux)
}

// page slices matched per the API's offset/limit query params, mimicking real full_json pagination.
func page(matched []stubAsset, offsetStr, limitStr string) []stubAsset {
	offset, _ := strconv.Atoi(offsetStr)
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = len(matched)
	}
	if offset > len(matched) {
		offset = len(matched)
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end]
}

// toFoundAsset builds the full_json representation of a stub, with each attribute's downloadLink
// pointing at base+"/file.zip".
func toFoundAsset(a stubAsset, base string) foundAsset {
	downloads := make([]downloadEntry, 0, len(a.Attributes))
	for _, attr := range a.Attributes {
		downloads = append(downloads, downloadEntry{
			DownloadLink: base + "/file.zip",
			FileName:     fmt.Sprintf("%s_%s.zip", a.AssetID, attr),
			Attribute:    attr,
			Filetype:     "zip",
		})
	}
	return foundAsset{
		AssetID:         a.AssetID,
		DisplayName:     a.DisplayName,
		DisplayCategory: a.DisplayCategory,
		Tags:            a.Tags,
		ShortLink:       "https://ambientcg.com/a/" + a.AssetID,
		DownloadFolders: map[string]downloadFolder{
			"default": {
				DownloadFiletypeCategories: map[string]filetypeCategory{
					"zip": {Downloads: downloads},
				},
			},
		},
	}
}

func TestSearchMapsAssetsAndPagination(t *testing.T) {
	assets := []stubAsset{
		{AssetID: "Bricks097", DisplayName: "Bricks 097", DisplayCategory: "Bricks", Tags: []string{"brick", "wall"}},
		{AssetID: "Metal032", DisplayName: "Metal 032", DisplayCategory: "Metal", Tags: []string{"metal"}},
		{AssetID: "Wood012", DisplayName: "Wood 012", DisplayCategory: "Wood", Tags: []string{"wood"}},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Assets) != 2 {
		t.Fatalf("len(Assets) = %d, want 2", len(res.Assets))
	}

	got := res.Assets[0]
	if got.ID != "ambientcg:Bricks097" {
		t.Errorf("Assets[0].ID = %q, want ambientcg:Bricks097", got.ID)
	}
	if got.Source != "Bricks" {
		t.Errorf("Assets[0].Source = %q, want Bricks", got.Source)
	}
	if got.Title != "Bricks 097" {
		t.Errorf("Assets[0].Title = %q, want \"Bricks 097\"", got.Title)
	}
	if got.Kind != assetcore.KindTexture {
		t.Errorf("Assets[0].Kind = %q, want texture", got.Kind)
	}
	if got.License != cc0License {
		t.Errorf("Assets[0].License = %+v, want %+v", got.License, cc0License)
	}
	if got.LandingURL != "https://ambientcg.com/a/Bricks097" {
		t.Errorf("Assets[0].LandingURL = %q", got.LandingURL)
	}

	// numberOfResults=3, offset=0, len(foundAssets)=3 (the stub ignores limit/offset itself and
	// always returns every match), so the mapped page of 2 leaves more upstream results.
	if res.NextCursor == "" {
		t.Fatal("NextCursor is empty, want a cursor for the remaining results")
	}

	res2, err := p.Search(t.Context(), assetcore.SearchOpts{Cursor: res.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("Search page 2: %v", err)
	}
	if len(res2.Assets) == 0 {
		t.Fatal("Search page 2 returned no assets")
	}
}

func TestSearchNextCursorEmptyOnLastPage(t *testing.T) {
	assets := []stubAsset{
		{AssetID: "Bricks097", DisplayName: "Bricks 097", DisplayCategory: "Bricks"},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want \"\"", res.NextCursor)
	}
}

func TestSearchMissingDisplayCategoryFallsBackToProviderName(t *testing.T) {
	assets := []stubAsset{
		{AssetID: "Bricks097", DisplayName: "Bricks 097"},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Assets) != 1 || res.Assets[0].Source != providerName {
		t.Fatalf("Assets = %+v, want single asset with Source %q", res.Assets, providerName)
	}
}

func TestFetchSelectsRequestedAttributeAndDefaults(t *testing.T) {
	assets := []stubAsset{
		{
			AssetID:         "Bricks097",
			DisplayName:     "Bricks 097",
			DisplayCategory: "Bricks",
			Attributes:      []string{"1K-JPG", "2K-PNG"},
		},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	// Default resolution/format (1K/JPG).
	blob, err := p.Fetch(t.Context(), "Bricks097", assetcore.TextureFetchOpts{})
	if err != nil {
		t.Fatalf("Fetch (defaults): %v", err)
	}
	if string(blob.Content) != zipBytes {
		t.Fatalf("Content = %q, want %q", blob.Content, zipBytes)
	}
	if blob.ContentType != "application/zip" {
		t.Fatalf("ContentType = %q, want application/zip", blob.ContentType)
	}
	if blob.Filename != "Bricks097_1K-JPG.zip" {
		t.Fatalf("Filename = %q, want Bricks097_1K-JPG.zip", blob.Filename)
	}
	if blob.Asset.ID != "ambientcg:Bricks097" {
		t.Fatalf("Asset.ID = %q, want ambientcg:Bricks097", blob.Asset.ID)
	}
	if blob.Asset.License != cc0License {
		t.Fatalf("Asset.License = %+v, want %+v", blob.Asset.License, cc0License)
	}
	if atomic.LoadInt32(&zipRequests) != 1 {
		t.Fatalf("zipRequests = %d, want 1", zipRequests)
	}

	// Explicit resolution/format selects the other attribute.
	blob2, err := p.Fetch(t.Context(), "Bricks097", assetcore.TextureFetchOpts{Resolution: "2K", Format: "PNG"})
	if err != nil {
		t.Fatalf("Fetch (2K/PNG): %v", err)
	}
	if blob2.Filename != "Bricks097_2K-PNG.zip" {
		t.Fatalf("Filename = %q, want Bricks097_2K-PNG.zip", blob2.Filename)
	}
	if atomic.LoadInt32(&zipRequests) != 2 {
		t.Fatalf("zipRequests = %d, want 2 after fetching a different attribute", zipRequests)
	}
}

func TestFetchCacheHitSkipsDownload(t *testing.T) {
	assets := []stubAsset{
		{AssetID: "Bricks097", DisplayName: "Bricks 097", DisplayCategory: "Bricks", Attributes: []string{"1K-JPG"}},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	if _, err := p.Fetch(t.Context(), "Bricks097", assetcore.TextureFetchOpts{}); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if atomic.LoadInt32(&zipRequests) != 1 {
		t.Fatalf("zipRequests after first fetch = %d, want 1", zipRequests)
	}

	if _, err := p.Fetch(t.Context(), "Bricks097", assetcore.TextureFetchOpts{}); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if atomic.LoadInt32(&zipRequests) != 1 {
		t.Fatalf("zipRequests after cache hit = %d, want still 1", zipRequests)
	}
}

func TestFetchUnknownAssetReturnsErrNotFound(t *testing.T) {
	var zipRequests int32
	srv := newStubServer(t, nil, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), "NotReal001", assetcore.TextureFetchOpts{})
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Fatalf("Fetch(unknown asset) error = %v, want assetcore.ErrNotFound", err)
	}
}

func TestFetchMissingAttributeReturnsErrNotFound(t *testing.T) {
	assets := []stubAsset{
		{AssetID: "Bricks097", DisplayName: "Bricks 097", DisplayCategory: "Bricks", Attributes: []string{"1K-JPG"}},
	}

	var zipRequests int32
	srv := newStubServer(t, assets, &zipRequests)
	defer srv.Close()
	withAPIBase(t, srv.URL)

	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), "Bricks097", assetcore.TextureFetchOpts{Resolution: "4K", Format: "EXR"})
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Fatalf("Fetch(missing attribute) error = %v, want assetcore.ErrNotFound", err)
	}
	if atomic.LoadInt32(&zipRequests) != 0 {
		t.Fatalf("zipRequests = %d, want 0 when no attribute matched", zipRequests)
	}
}

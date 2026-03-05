package shopify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/shopify"
)

func testConfig(baseURL string) shopify.Config {
	return shopify.Config{
		ID:      datasource.Hiyou,
		Name:    "HiYoU",
		BaseURL: baseURL,
	}
}

func TestSearchProducts(t *testing.T) {
	srv := jsonFixtureServer(t, "testdata/search.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	products, err := ds.SearchProducts(context.Background(), "rice")
	if err != nil {
		t.Fatal(err)
	}

	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "name", p.Name, "Golden Bowl Thai Hom Mali Rice 1kg")
	assertFloat(t, p.Price, 2.85)
	assertString(t, "id", p.ID, "golden-bowl-thai-hom-mali-rice-1kg")
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Hiyou))
	assertString(t, "currency", p.Currency, "GBP")
	if !p.Available {
		t.Error("expected product to be available")
	}
	if p.ImageURL == "" {
		t.Error("expected non-empty image URL")
	}
	if !strings.HasPrefix(p.URL, srv.URL) {
		t.Errorf("expected URL to start with server URL, got %q", p.URL)
	}

	// Second product: unavailable, image falls back to featured_image.
	p2 := products[1]
	if p2.Available {
		t.Error("expected second product to be unavailable")
	}
	if p2.ImageURL == "" {
		t.Error("expected featured_image fallback for empty image")
	}
}

func TestGetProductDetails(t *testing.T) {
	srv := jsonFixtureServer(t, "testdata/product.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	p, err := ds.GetProductDetails(
		context.Background(), "golden-bowl-thai-hom-mali-rice-1kg",
	)
	if err != nil {
		t.Fatal(err)
	}

	assertString(t, "name", p.Name, "Golden Bowl Thai Hom Mali Rice 1kg")
	assertFloat(t, p.Price, 2.85)
	assertString(t, "weight", p.Weight, "1kg")
	assertString(t, "id", p.ID, "golden-bowl-thai-hom-mali-rice-1kg")
	if p.ImageURL == "" {
		t.Error("expected non-empty image URL")
	}
}

func TestBrowseCategories(t *testing.T) {
	srv := jsonFixtureServer(t, "testdata/collections.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	categories, err := ds.BrowseCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
	assertString(t, "category 0 name", categories[0].Name, "Summer Sale")
	assertString(t, "category 0 supermarket",
		string(categories[0].Supermarket), string(datasource.Hiyou))
	if !strings.Contains(categories[0].URL, "/collections/summer-sale") {
		t.Errorf("unexpected category URL: %q", categories[0].URL)
	}
}

func TestHTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}),
	)
	defer srv.Close()

	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	_, err := ds.SearchProducts(context.Background(), "rice")
	if err == nil {
		t.Fatal("expected error from 403, got nil")
	}
}

// Test helpers.

func jsonFixtureServer(t *testing.T, fixturePath string) *httptest.Server {
	t.Helper()
	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // Test fixture path.
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		}),
	)
	t.Cleanup(srv.Close)
	return srv
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("got %f, want %f", got, want)
	}
}

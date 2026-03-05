package sainsburys_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/sainsburys"
)

func TestSearchProducts(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_search.json")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasourceWithURL(srv.URL)

	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}

	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "name", p.Name,
		"Sainsbury's British Semi Skimmed Milk 2.27L")
	assertFloat(t, p.Price, 1.45)
	assertString(t, "supermarket",
		string(p.Supermarket), string(datasource.Sainsburys))
	assertString(t, "id", p.ID, "7878921")

	assertString(t, "promotion",
		products[1].Promotion, "Nectar Price £1.70")
}

func TestGetProductDetails(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_product.json")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasourceWithURL(srv.URL)
	p, err := ds.GetProductDetails(context.Background(), "7878921")
	if err != nil {
		t.Fatal(err)
	}

	assertString(t, "name", p.Name,
		"Sainsbury's British Semi Skimmed Milk 2.27L")
	assertFloat(t, p.Price, 1.45)
}

func TestBrowseCategories(t *testing.T) {
	srv := jsonFixtureServer(t, "testdata/sainsburys_categories.json")
	ds := sainsburys.NewDatasourceWithURL(srv.URL)
	testBrowseCategories(t, ds, 2, "Meat & Fish", datasource.Sainsburys)
}

func TestHTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasourceWithURL(srv.URL)

	_, err := ds.SearchProducts(context.Background(), "milk")
	if err == nil {
		t.Fatal("expected error from 403, got nil")
	}
}

func TestCookiesAndWCAuthTokenSent(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_search.json")
	if err != nil {
		t.Fatal(err)
	}

	var gotCookies string
	var gotAuthToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookies = r.Header.Get("Cookie")
		gotAuthToken = r.Header.Get("wcauthtoken")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	cookies := []*http.Cookie{
		{Name: "session_id", Value: "abc123"},
		{Name: "WC_AUTHENTICATION_12345", Value: "my-auth-token"},
	}
	ds := sainsburys.NewDatasourceWithURL(srv.URL)
	ds.SetCookies(cookies)

	_, err = ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(gotCookies, "session_id=abc123") {
		t.Errorf("expected session_id cookie, got Cookie header: %q", gotCookies)
	}
	if !strings.Contains(gotCookies, "WC_AUTHENTICATION_12345=my-auth-token") {
		t.Errorf("expected WC_AUTHENTICATION cookie, got Cookie header: %q", gotCookies)
	}
	expectedWCAuth := "my-auth-token"
	if gotAuthToken != expectedWCAuth {
		t.Errorf("wcauthtoken header = %q, want %q", gotAuthToken, expectedWCAuth)
	}
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()
	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}
	assertSearchResults(t, products, "milk")
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()

	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 {
		t.Fatal("no search results to look up")
	}

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name == "" {
		t.Error("empty product name")
	}
	if p.Price <= 0 {
		t.Errorf("expected positive price, got %f", p.Price)
	}
	if p.URL == "" {
		t.Error("empty product URL")
	}
}

func TestBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()

	categories, err := ds.BrowseCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) == 0 {
		t.Fatal("expected categories")
	}
	for _, c := range categories {
		if c.Name == "" {
			t.Error("empty category name")
		}
		if c.Supermarket != datasource.Sainsburys {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Sainsburys)
		}
		if c.URL == "" {
			t.Error("empty category URL")
		}
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

func testBrowseCategories(
	t *testing.T,
	ds datasource.Datasource,
	expectedCount int,
	expectedName string,
	expectedSupermarket datasource.SupermarketID,
) {
	t.Helper()
	categories, err := ds.BrowseCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != expectedCount {
		t.Fatalf("expected %d categories, got %d",
			expectedCount, len(categories))
	}
	assertString(t, "category 0 name",
		categories[0].Name, expectedName)
	assertString(t, "category 0 supermarket",
		string(categories[0].Supermarket),
		string(expectedSupermarket))
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

func assertSearchResults(t *testing.T, products []datasource.Product, query string) {
	t.Helper()
	if len(products) == 0 {
		t.Fatal("expected results")
	}
	relevant := 0
	for _, p := range products {
		if p.Name == "" {
			t.Error("empty product name")
		}
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			relevant++
		}
	}
	minRelevant := len(products) / 4
	if minRelevant < 1 {
		minRelevant = 1
	}
	if relevant < minRelevant {
		t.Errorf("only %d/%d results contain %q in their name (want at least %d)",
			relevant, len(products), query, minRelevant)
		for i, p := range products {
			if i >= 5 {
				break
			}
			t.Logf("  [%d] %s", i, p.Name)
		}
	}
}

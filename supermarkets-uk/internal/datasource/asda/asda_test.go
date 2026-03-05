package asda_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/asda"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

func TestParseAlgoliaResults(t *testing.T) {
	f := openTestFile(t, "testdata/asda_search.json")
	products, err := asda.ParseAlgoliaResults(f)
	if err != nil {
		t.Fatal(err)
	}

	if len(products) < 3 {
		t.Fatalf("expected at least 3 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Asda))
	assertString(t, "currency", p.Currency, "GBP")
	if p.Name == "" {
		t.Error("expected non-empty product name")
	}
	if p.Price == 0 {
		t.Error("expected non-zero price")
	}
	if p.ID == "" {
		t.Error("expected non-empty product ID")
	}
	if p.URL == "" {
		t.Error("expected non-empty product URL")
	}
	if p.ImageURL == "" {
		t.Error("expected non-empty image URL")
	}
	if !strings.Contains(p.URL, "/groceries/product/") {
		t.Errorf("URL %q does not contain /groceries/product/", p.URL)
	}
	if p.PricePerUnit == "" {
		t.Error("expected non-empty price per unit")
	}
}

func TestParseSearchResults(t *testing.T) {
	products := parseSearchFile(t, "testdata/asda_search.html", asda.ParseSearchResults)

	if len(products) != 3 {
		t.Fatalf("expected 3 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Asda))
	assertString(t, "currency", p.Currency, "GBP")
	if p.Name == "" {
		t.Error("expected non-empty product name")
	}
	if p.Price == 0 {
		t.Error("expected non-zero price")
	}
	if p.ID == "" {
		t.Error("expected non-empty product ID")
	}
	if p.URL == "" {
		t.Error("expected non-empty product URL")
	}
	if p.ImageURL == "" {
		t.Error("expected non-empty image URL")
	}
	if !strings.Contains(p.URL, "/groceries/product/") {
		t.Errorf("URL %q does not contain /groceries/product/", p.URL)
	}
}

func TestParseCategories(t *testing.T) {
	categories := parseCategoryFile(t, "testdata/asda_categories.html", asda.ParseCategories)

	if len(categories) < 15 {
		t.Fatalf("expected at least 15 categories, got %d", len(categories))
	}

	assertString(t, "supermarket", string(categories[0].Supermarket), string(datasource.Asda))
	if categories[0].Name == "" {
		t.Error("expected non-empty category name")
	}
	if categories[0].URL == "" {
		t.Error("expected non-empty category URL")
	}
}

func TestParseProductPage(t *testing.T) {
	f := openTestFile(t, "testdata/asda_product.html")
	p, err := asda.ParseProductPage(f)
	if err != nil {
		t.Fatal(err)
	}

	assertString(t, "name", p.Name, "ASDA British Milk Semi Skimmed 4 Pints")
	if p.Price != 1.65 {
		t.Errorf("price = %f, want 1.65", p.Price)
	}
	if p.PricePerUnit == "" {
		t.Error("expected non-empty price per unit")
	}
	assertString(t, "weight", p.Weight, "4 pint")
	if p.ImageURL == "" {
		t.Error("expected non-empty image URL")
	}
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := asda.NewDatasource(browser)
	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 {
		t.Fatal("expected results")
	}
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := asda.NewDatasource(browser)

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
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := asda.NewDatasource(browser)

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
		if c.Supermarket != datasource.Asda {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Asda)
		}
		if c.URL == "" {
			t.Error("empty category URL")
		}
	}
}

// Test helpers.

func openTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // Test fixture paths are not user-controlled.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func parseSearchFile(
	t *testing.T,
	path string,
	fn func(io.Reader) ([]datasource.Product, error),
) []datasource.Product {
	t.Helper()
	f := openTestFile(t, path)
	products, err := fn(f)
	if err != nil {
		t.Fatal(err)
	}
	return products
}

func parseCategoryFile(
	t *testing.T,
	path string,
	fn func(io.Reader) ([]datasource.Category, error),
) []datasource.Category {
	t.Helper()
	f := openTestFile(t, path)
	categories, err := fn(f)
	if err != nil {
		t.Fatal(err)
	}
	return categories
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

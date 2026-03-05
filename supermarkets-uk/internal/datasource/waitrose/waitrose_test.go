package waitrose_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/waitrose"
)

func TestParseSearchResults(t *testing.T) {
	products := parseSearchFile(t, "testdata/waitrose_search.html", waitrose.ParseSearchResults)

	if len(products) < 10 {
		t.Fatalf("expected at least 10 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Waitrose))
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
	if !strings.Contains(p.URL, "/ecom/products/") {
		t.Errorf("URL %q does not contain /ecom/products/", p.URL)
	}
	if strings.Contains(strings.ToLower(p.Name), "price per unit") {
		t.Errorf("product name should not contain 'price per unit': %q", p.Name)
	}
}

func TestParsePencePrice(t *testing.T) {
	products := parseSearchFile(t, "testdata/waitrose_search.html", waitrose.ParseSearchResults)

	// Product 8 in the fixture has a "95p" price (no £ sign).
	if len(products) <= 8 {
		t.Fatalf("expected at least 9 products, got %d", len(products))
	}
	p := products[8]
	if p.Price < 0.01 {
		t.Errorf("expected pence-only price to parse, got %.2f", p.Price)
	}
	if p.Price != 0.95 {
		t.Errorf("expected price 0.95, got %.2f", p.Price)
	}
}

func TestParseCategories(t *testing.T) {
	f := openTestFile(t, "testdata/waitrose_categories.html")
	categories, err := waitrose.ParseCategories(f)
	if err != nil {
		t.Fatal(err)
	}

	if len(categories) < 18 {
		t.Fatalf("expected at least 18 categories, got %d", len(categories))
	}

	// Verify first category.
	c := categories[0]
	assertString(t, "supermarket", string(c.Supermarket), string(datasource.Waitrose))
	if c.Name == "" {
		t.Error("expected non-empty category name")
	}
	if c.URL == "" {
		t.Error("expected non-empty category URL")
	}
	if c.ID == "" {
		t.Error("expected non-empty category ID")
	}
	if !strings.Contains(c.URL, "/ecom/shop/browse/groceries/") {
		t.Errorf("URL %q does not contain /ecom/shop/browse/groceries/", c.URL)
	}

	// Verify no subcategories leaked in (they have extra path segments).
	for _, cat := range categories {
		suffix := strings.TrimPrefix(cat.URL, "https://www.waitrose.com/ecom/shop/browse/groceries/")
		if strings.Contains(suffix, "/") {
			t.Errorf("category %q URL %q looks like a subcategory", cat.Name, cat.URL)
		}
	}

	// Check that well-known categories are present.
	names := make(map[string]bool)
	for _, cat := range categories {
		names[cat.Name] = true
	}
	for _, want := range []string{"Frozen", "Bakery", "Household", "New"} {
		if !names[want] {
			t.Errorf("expected to find category %q", want)
		}
	}
}

func TestParseProductPage(t *testing.T) {
	f := openTestFile(t, "testdata/waitrose_product.html")
	p, err := waitrose.ParseProductPage(f)
	if err != nil {
		t.Fatal(err)
	}

	assertString(t, "name", p.Name, "Essential British Free Range Semi-Skimmed Milk 4 Pints")
	if p.Price != 1.75 {
		t.Errorf("price = %f, want 1.75", p.Price)
	}
	assertString(t, "pricePerUnit", p.PricePerUnit, "77p/litre")
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := waitrose.NewDatasource(browser)
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
	ds := waitrose.NewDatasource(browser)

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
	ds := waitrose.NewDatasource(browser)

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
		if c.Supermarket != datasource.Waitrose {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Waitrose)
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

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

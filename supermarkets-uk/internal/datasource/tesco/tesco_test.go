package tesco_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/tesco"
)

func TestParseSearchResults(t *testing.T) {
	products := parseSearchFile(t, "testdata/tesco_search.html", tesco.ParseSearchResults)

	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "name", p.Name, "Tesco Semi Skimmed Milk 2.272L/4 Pints")
	assertFloat(t, p.Price, 1.65)
	assertString(t, "pricePerUnit", p.PricePerUnit, "72.6p/litre")
	assertString(t, "promotion", p.Promotion, "Clubcard Price")
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Tesco))
	assertString(t, "id", p.ID, "123456789")
	assertString(t, "currency", p.Currency, "GBP")

	p2 := products[1]
	assertString(t, "name", p2.Name, "Cravendale Semi Skimmed Milk 2L")
	assertFloat(t, p2.Price, 1.95)
	assertString(t, "id", p2.ID, "987654321")
}

func TestParseProductPage(t *testing.T) {
	p := parseProductFile(t, "testdata/tesco_product.html", tesco.ParseProductPage)

	assertString(t, "name", p.Name, "Tesco British Semi Skimmed Milk 2.272L, 4 Pints")
	assertFloat(t, p.Price, 1.65)
	assertString(t, "pricePerUnit", p.PricePerUnit, "£0.73/litre")
}

func TestParseCategories(t *testing.T) {
	categories := parseCategoryFile(t, "testdata/tesco_categories.html", tesco.ParseCategories)

	if len(categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(categories))
	}

	assertString(t, "category 0 name", categories[0].Name, "Fresh Food")
	assertString(t, "category 0 supermarket", string(categories[0].Supermarket), string(datasource.Tesco))
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser)
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
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser)
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
	ds := tesco.NewDatasource(browser)

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
		if c.Supermarket != datasource.Tesco {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Tesco)
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

func parseProductFile(
	t *testing.T,
	path string,
	fn func(io.Reader) (*datasource.Product, error),
) *datasource.Product {
	t.Helper()
	f := openTestFile(t, path)
	p, err := fn(f)
	if err != nil {
		t.Fatal(err)
	}
	return p
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

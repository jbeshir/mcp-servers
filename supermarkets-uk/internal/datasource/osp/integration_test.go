package osp_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
)

func TestOcadoSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()
	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}
	assertSearchResults(t, products, "milk")
}

func TestMorrisonsSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()
	products, err := ds.SearchProducts(context.Background(), "milk")
	if err != nil {
		t.Fatal(err)
	}
	assertSearchResults(t, products, "milk")
}

func TestOcadoProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()

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

func TestMorrisonsProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()

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

func TestOcadoBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()

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
		if c.Supermarket != datasource.Ocado {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Ocado)
		}
		if c.URL == "" {
			t.Error("empty category URL")
		}
	}
}

func TestMorrisonsBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()

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
		if c.Supermarket != datasource.Morrisons {
			t.Errorf("supermarket = %q, want %q", c.Supermarket, datasource.Morrisons)
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

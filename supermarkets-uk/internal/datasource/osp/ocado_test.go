package osp_test

import (
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
)

func TestParseOcadoSearchResults(t *testing.T) {
	products := parseSearchFile(t, "testdata/ocado_search.html", osp.ParseOcadoSearchResults)

	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	p := products[0]
	assertString(t, "name", p.Name, "Cravendale Filtered Fresh Whole Milk Fresher for Longer")
	assertFloat(t, p.Price, 2.70)
	assertString(t, "supermarket",
		string(p.Supermarket), string(datasource.Ocado))
	assertString(t, "id", p.ID, "24577011")
}

func TestParseOcadoProductPage(t *testing.T) {
	p := parseProductFile(t, "testdata/ocado_product.html", osp.ParseOcadoProductPage)

	assertString(t, "name", p.Name, "Cravendale Filtered Fresh Whole Milk Fresher for Longer")
	assertFloat(t, p.Price, 2.70)
}

func TestParseOcadoCategories(t *testing.T) {
	categories := parseCategoryFile(t, "testdata/ocado_categories.html", osp.ParseOcadoCategories)

	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}

	assertString(t, "category 0 name", categories[0].Name, "Fresh & Chilled Food")
	assertString(t, "category 0 supermarket",
		string(categories[0].Supermarket), string(datasource.Ocado))
}

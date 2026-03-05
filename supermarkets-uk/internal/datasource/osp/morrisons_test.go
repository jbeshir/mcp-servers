package osp_test

import (
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
)

func TestParseMorrisonsSearchResults(t *testing.T) {
	products := parseSearchFile(t, "testdata/morrisons_search.html", osp.ParseMorrisonsSearchResults)

	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}

	p := products[0]
	assertString(t, "name", p.Name, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g")
	assertFloat(t, p.Price, 2.80)
	assertString(t, "weight", p.Weight, "115g")
	assertString(t, "supermarket", string(p.Supermarket), string(datasource.Morrisons))
}

func TestParseMorrisonsProductPage(t *testing.T) {
	p := parseProductFile(t, "testdata/morrisons_product.html", osp.ParseMorrisonsProductPage)

	assertString(t, "name", p.Name, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g")
	assertFloat(t, p.Price, 2.80)
}

func TestParseMorrisonsCategories(t *testing.T) {
	categories := parseCategoryFile(t, "testdata/morrisons_categories.html", osp.ParseMorrisonsCategories)

	if len(categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(categories))
	}

	assertString(t, "category 0 name", categories[0].Name, "Fruit, Veg & Flowers")
	assertString(t, "category 0 supermarket", string(categories[0].Supermarket), string(datasource.Morrisons))
}

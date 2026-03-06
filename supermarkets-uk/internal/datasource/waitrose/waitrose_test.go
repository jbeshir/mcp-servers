package waitrose_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/waitrose"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseSearchResults(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/waitrose_search.html", waitrose.ParseSearchResults)

	require.GreaterOrEqual(t, len(products), 10)

	p := products[0]
	assert.Equal(t, datasource.Waitrose, p.Supermarket)
	assert.Equal(t, "GBP", p.Currency)
	assert.NotEmpty(t, p.Name)
	assert.NotZero(t, p.Price)
	assert.NotEmpty(t, p.ID)
	assert.NotEmpty(t, p.URL)
	assert.NotEmpty(t, p.ImageURL)
	assert.Contains(t, p.URL, "/ecom/products/")
	assert.NotContains(t, strings.ToLower(p.Name), "price per unit")
}

func TestParsePencePrice(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/waitrose_search.html", waitrose.ParseSearchResults)

	// Product 8 in the fixture has a "95p" price (no £ sign).
	require.Greater(t, len(products), 8)
	p := products[8]
	assert.Equal(t, 0.95, p.Price)
}

func TestParseCategories(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/waitrose_categories.html")
	categories, err := waitrose.ParseCategories(f)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(categories), 18)

	// Verify first category.
	c := categories[0]
	assert.Equal(t, datasource.Waitrose, c.Supermarket)
	assert.NotEmpty(t, c.Name)
	assert.NotEmpty(t, c.URL)
	assert.NotEmpty(t, c.ID)
	assert.Contains(t, c.URL, "/ecom/shop/browse/groceries/")

	// Verify no subcategories leaked in (they have extra path segments).
	for _, cat := range categories {
		suffix := strings.TrimPrefix(cat.URL, "https://www.waitrose.com/ecom/shop/browse/groceries/")
		assert.NotContains(t, suffix, "/", "category %q URL %q looks like a subcategory", cat.Name, cat.URL)
	}

	// Check that well-known categories are present.
	names := make(map[string]bool)
	for _, cat := range categories {
		names[cat.Name] = true
	}
	for _, want := range []string{"Frozen", "Bakery", "Household", "New"} {
		assert.True(t, names[want], "expected to find category %q", want)
	}
}

func TestParseProductPage(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/waitrose_product.html")
	p, err := waitrose.ParseProductPage(f)
	require.NoError(t, err)

	assert.Equal(t, "Essential British Free Range Semi-Skimmed Milk 4 Pints", p.Name)
	assert.Equal(t, 1.75, p.Price)
	assert.Equal(t, "77p/litre", p.PricePerUnit)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
	assert.NotEmpty(t, p.Nutrition.PerPortion["Energy"])
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := waitrose.NewDatasource(browser)
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products)
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := waitrose.NewDatasource(browser)

	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name)
	assert.Positive(t, p.Price)
	assert.NotEmpty(t, p.URL)
	assert.NotEmpty(t, p.Description)
	require.NotNil(t, p.Nutrition, "expected nutrition info")
	assert.NotEmpty(t, p.Nutrition.Per100g, "expected per-100g nutrition data")
}

func TestBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := waitrose.NewDatasource(browser)

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Waitrose, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

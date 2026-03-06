package tesco_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/tesco"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseSearchResults(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/tesco_search.html", tesco.ParseSearchResults)

	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "Tesco Semi Skimmed Milk 2.272L/4 Pints", p.Name)
	assert.Equal(t, 1.65, p.Price)
	assert.Equal(t, "72.6p/litre", p.PricePerUnit)
	assert.Equal(t, "Clubcard Price", p.Promotion)
	assert.Equal(t, datasource.Tesco, p.Supermarket)
	assert.Equal(t, "123456789", p.ID)
	assert.Equal(t, "GBP", p.Currency)

	p2 := products[1]
	assert.Equal(t, "Cravendale Semi Skimmed Milk 2L", p2.Name)
	assert.Equal(t, 1.95, p2.Price)
	assert.Equal(t, "987654321", p2.ID)
}

func TestParseProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/tesco_product.html", tesco.ParseProductPage)

	assert.Equal(t, "Tesco British Semi Skimmed Milk 2.272L, 4 Pints", p.Name)
	assert.Equal(t, 1.65, p.Price)
	assert.Equal(t, "£0.73/litre", p.PricePerUnit)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
	assert.NotEmpty(t, p.Nutrition.PerPortion["Energy"])
}

func TestParseCategories(t *testing.T) {
	categories := testutil.ParseCategoryFile(t, "testdata/tesco_categories.html", tesco.ParseCategories)

	require.Len(t, categories, 3)
	assert.Equal(t, "Fresh Food", categories[0].Name)
	assert.Equal(t, datasource.Tesco, categories[0].Supermarket)
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser)
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser)
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name)
	assert.Positive(t, p.Price)
	assert.NotEmpty(t, p.URL)
	assert.NotEmpty(t, p.Description)
	assert.NotEmpty(t, p.Ingredients)
	require.NotNil(t, p.Nutrition, "expected nutrition info")
	assert.NotEmpty(t, p.Nutrition.Per100g, "expected per-100g nutrition data")
}

func TestBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser)

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Tesco, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

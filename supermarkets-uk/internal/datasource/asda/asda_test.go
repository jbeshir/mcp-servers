package asda_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/asda"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseAlgoliaResults(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/asda_search.json")
	products, err := asda.ParseAlgoliaResults(f)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(products), 3)

	p := products[0]
	assert.Equal(t, datasource.Asda, p.Supermarket)
	assert.Equal(t, "GBP", p.Currency)
	assert.NotEmpty(t, p.Name)
	assert.NotZero(t, p.Price)
	assert.NotEmpty(t, p.ID)
	assert.NotEmpty(t, p.URL)
	assert.NotEmpty(t, p.ImageURL)
	assert.Contains(t, p.URL, "/groceries/product/")
	assert.NotEmpty(t, p.PricePerUnit)
	assert.NotEmpty(t, p.DietaryInfo, "expected dietary info from NUTRITIONAL_INFO flags")
	assert.Contains(t, p.DietaryInfo, "Vegetarian")
}

func TestParseCategories(t *testing.T) {
	categories := testutil.ParseCategoryFile(t, "testdata/asda_categories.html", asda.ParseCategories)

	require.GreaterOrEqual(t, len(categories), 15)
	assert.Equal(t, datasource.Asda, categories[0].Supermarket)
	assert.NotEmpty(t, categories[0].Name)
	assert.NotEmpty(t, categories[0].URL)
}

func TestParseProductPage(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/asda_product.html")
	p, err := asda.ParseProductPage(f)
	require.NoError(t, err)

	assert.Equal(t, "ASDA British Milk Semi Skimmed 4 Pints", p.Name)
	assert.Equal(t, 1.65, p.Price)
	assert.NotEmpty(t, p.PricePerUnit)
	assert.Equal(t, "4 pint", p.Weight)
	assert.NotEmpty(t, p.ImageURL)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := asda.NewDatasource(browser)
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
	ds := asda.NewDatasource(browser)

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
	ds := asda.NewDatasource(browser)

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Asda, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

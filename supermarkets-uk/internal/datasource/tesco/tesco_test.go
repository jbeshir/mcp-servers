package tesco_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/auth"
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
	assert.InDelta(t, 1.65, p.Price, 0.001)
	assert.Equal(t, "72.6p/litre", p.PricePerUnit)
	assert.Equal(t, "Clubcard Price", p.Promotion)
	assert.Equal(t, datasource.Tesco, p.Supermarket)
	assert.Equal(t, "123456789", p.ID)
	assert.Equal(t, "GBP", p.Currency)

	p2 := products[1]
	assert.Equal(t, "Cravendale Semi Skimmed Milk 2L", p2.Name)
	assert.InDelta(t, 1.95, p2.Price, 0.001)
	assert.Equal(t, "987654321", p2.ID)
}

func TestParseProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/tesco_product.html", tesco.ParseProductPage)

	assert.Equal(t, "Tesco British Semi Skimmed Milk 2.272L, 4 Pints", p.Name)
	assert.InDelta(t, 1.65, p.Price, 0.001)
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

func TestParseOrderHistory(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/tesco_orders.html")
	result, err := tesco.ParseOrderHistory(f)
	require.NoError(t, err)

	assert.Equal(t, datasource.Tesco, result.Supermarket)
	require.NotNil(t, result.Total)
	assert.Equal(t, 25, *result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
	require.Len(t, result.Orders, 2)

	// First order: delivered.
	o1 := result.Orders[0]
	assert.Equal(t, "1234-5678-90", o1.ID)
	assert.Equal(t, "Delivered", o1.Status)
	assert.InDelta(t, 85.50, o1.TotalPrice, 0.001)
	assert.Equal(t, 5, o1.TotalItems)
	assert.Equal(t, "delivery", o1.ShoppingMethod)
	assert.Equal(t, "GBP", o1.Currency)
	assert.Equal(t, "2026-03-01", o1.Date)
	assert.Contains(t, o1.DeliverySlot, "3:00pm")
	assert.Contains(t, o1.DeliverySlot, "4:00pm")
	require.Len(t, o1.Items, 3)
	assert.Equal(t, "Semi Skimmed Milk 2L", o1.Items[0].Name)
	assert.Equal(t, "111111", o1.Items[0].ProductID)
	assert.Equal(t, 2, o1.Items[0].Quantity)
	assert.Equal(t, "Wholemeal Bread 800g", o1.Items[1].Name)
	assert.Equal(t, 1, o1.Items[1].Quantity)
	assert.Equal(t, "Free Range Eggs 6 Pack", o1.Items[2].Name)
	assert.Equal(t, 4, o1.Items[2].Quantity)

	// Second order: cancelled.
	o2 := result.Orders[1]
	assert.Equal(t, "9876-5432-10", o2.ID)
	assert.Equal(t, "Cancelled", o2.Status)
	assert.InDelta(t, 42.00, o2.TotalPrice, 0.001)
	assert.Equal(t, "collection", o2.ShoppingMethod)
	require.Len(t, o2.Items, 1)
	assert.Equal(t, "Cheddar Cheese 400g", o2.Items[0].Name)
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser, &http.Client{})
	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	browser := scraper.NewBrowser()
	defer browser.Close()
	ds := tesco.NewDatasource(browser, &http.Client{})
	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(t.Context(), products[0].ID)
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
	ds := tesco.NewDatasource(browser, &http.Client{})

	categories, err := ds.BrowseCategories(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Tesco, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

func TestParseBasket(t *testing.T) {
	f := testutil.OpenTestFile(t, "testdata/tesco_trolley.html")
	basket, err := tesco.ParseBasket(f)
	require.NoError(t, err)

	assert.Equal(t, datasource.Tesco, basket.Supermarket)
	assert.Equal(t, "GBP", basket.Currency)
	assert.InDelta(t, 12.50, basket.TotalPrice, 0.001)
	assert.Equal(t, 5, basket.TotalItems)
	require.Len(t, basket.Items, 2)

	i1 := basket.Items[0]
	assert.Equal(t, "111111", i1.ProductID)
	assert.Equal(t, "Semi Skimmed Milk 2L", i1.Name)
	assert.Equal(t, 2, i1.Quantity)
	assert.InDelta(t, 3.30, i1.Cost, 0.001)
	assert.InDelta(t, 1.65, i1.Price, 0.001)
	assert.Equal(t, "Clubcard Price", i1.Promotion)

	i2 := basket.Items[1]
	assert.Equal(t, "222222", i2.ProductID)
	assert.Equal(t, "Wholemeal Bread 800g", i2.Name)
	assert.Equal(t, 3, i2.Quantity)
	assert.InDelta(t, 9.20, i2.Cost, 0.001)
	assert.InDelta(t, 1.10, i2.Price, 0.001)
	assert.Empty(t, i2.Promotion)
}

func TestGetBasketIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := tescoWithCookies(t)
	basket, err := ds.GetBasket(t.Context())
	require.NoError(t, err)
	assert.Equal(t, datasource.Tesco, basket.Supermarket)
	assert.Equal(t, "GBP", basket.Currency)
	t.Logf("Basket: %d items, total £%.2f", basket.TotalItems, basket.TotalPrice)
	for _, item := range basket.Items {
		assert.NotEmpty(t, item.ProductID)
		assert.NotEmpty(t, item.Name)
		assert.Positive(t, item.Quantity)
		t.Logf("  %dx %s (£%.2f)", item.Quantity, item.Name, item.Cost)
	}
}

func tescoWithCookies(t *testing.T) *tesco.Datasource {
	t.Helper()
	cookieDir, err := auth.DefaultCookieDir()
	require.NoError(t, err)
	store, err := auth.NewCookieStore(cookieDir)
	require.NoError(t, err)
	cookies, err := store.Load(datasource.Tesco)
	require.NoError(t, err)
	if len(cookies) == 0 {
		t.Skip("no cached cookies for tesco")
	}
	ds := tesco.NewDatasource(scraper.NewBrowser(), &http.Client{})
	ds.SetCookies(cookies)
	return ds
}

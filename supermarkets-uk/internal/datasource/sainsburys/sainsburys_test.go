package sainsburys_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/sainsburys"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestSearchProducts(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_search.json")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasource(sainsburys.Config{BaseURL: srv.URL}, srv.Client())

	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "Sainsbury's British Semi Skimmed Milk 2.27L", p.Name)
	assert.InDelta(t, 1.45, p.Price, 0.001)
	assert.Equal(t, datasource.Sainsburys, p.Supermarket)
	assert.Equal(t, "7878921", p.ID)
	assert.Equal(t, "Nectar Price £1.70", products[1].Promotion)
}

func TestGetProductDetails(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_product.json")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasource(sainsburys.Config{BaseURL: srv.URL}, srv.Client())
	p, err := ds.GetProductDetails(t.Context(), "7878921")
	require.NoError(t, err)

	assert.Equal(t, "Sainsbury's British Semi Skimmed Milk 2.27L", p.Name)
	assert.InDelta(t, 1.45, p.Price, 0.001)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
	assert.NotEmpty(t, p.Nutrition.PerPortion["Energy"])
	assert.Equal(t, "200ml", p.Nutrition.PortionSize)
}

func TestBrowseCategories(t *testing.T) {
	srv := testutil.JSONFixtureServer(t, "testdata/sainsburys_categories.json")
	ds := sainsburys.NewDatasource(sainsburys.Config{BaseURL: srv.URL}, srv.Client())

	categories, err := ds.BrowseCategories(t.Context())
	require.NoError(t, err)
	require.Len(t, categories, 2)
	assert.Equal(t, "Meat & Fish", categories[0].Name)
	assert.Equal(t, datasource.Sainsburys, categories[0].Supermarket)
}

func TestHTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ds := sainsburys.NewDatasource(sainsburys.Config{BaseURL: srv.URL}, srv.Client())

	_, err := ds.SearchProducts(t.Context(), "milk")
	assert.Error(t, err)
}

func TestCookiesAndWCAuthTokenSent(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sainsburys_search.json")
	require.NoError(t, err)

	var gotCookies string
	var gotAuthToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookies = r.Header.Get("Cookie")
		gotAuthToken = r.Header.Get("wcauthtoken")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	cookies := []*http.Cookie{
		{Name: "session_id", Value: "abc123"},
		{Name: "WC_AUTHENTICATION_12345", Value: "my-auth-token"},
	}
	ds := sainsburys.NewDatasource(sainsburys.Config{BaseURL: srv.URL}, srv.Client())
	ds.SetCookies(cookies)

	_, err = ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)

	assert.Contains(t, gotCookies, "session_id=abc123")
	assert.Contains(t, gotCookies, "WC_AUTHENTICATION_12345=my-auth-token")
	assert.Equal(t, "my-auth-token", gotAuthToken)
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource(sainsburys.Config{}, &http.Client{})
	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource(sainsburys.Config{}, &http.Client{})

	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(t.Context(), products[0].ID)
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
	ds := sainsburys.NewDatasource(sainsburys.Config{}, &http.Client{})

	categories, err := ds.BrowseCategories(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Sainsburys, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

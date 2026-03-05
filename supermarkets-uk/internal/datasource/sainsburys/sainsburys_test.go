package sainsburys_test

import (
	"context"
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

	ds := sainsburys.NewDatasourceWithURL(srv.URL)

	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "Sainsbury's British Semi Skimmed Milk 2.27L", p.Name)
	assert.Equal(t, 1.45, p.Price)
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

	ds := sainsburys.NewDatasourceWithURL(srv.URL)
	p, err := ds.GetProductDetails(context.Background(), "7878921")
	require.NoError(t, err)

	assert.Equal(t, "Sainsbury's British Semi Skimmed Milk 2.27L", p.Name)
	assert.Equal(t, 1.45, p.Price)
}

func TestBrowseCategories(t *testing.T) {
	srv := testutil.JSONFixtureServer(t, "testdata/sainsburys_categories.json")
	ds := sainsburys.NewDatasourceWithURL(srv.URL)

	categories, err := ds.BrowseCategories(context.Background())
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

	ds := sainsburys.NewDatasourceWithURL(srv.URL)

	_, err := ds.SearchProducts(context.Background(), "milk")
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
	ds := sainsburys.NewDatasourceWithURL(srv.URL)
	ds.SetCookies(cookies)

	_, err = ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)

	assert.Contains(t, gotCookies, "session_id=abc123")
	assert.Contains(t, gotCookies, "WC_AUTHENTICATION_12345=my-auth-token")
	assert.Equal(t, "my-auth-token", gotAuthToken)
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()

	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name)
	assert.Positive(t, p.Price)
	assert.NotEmpty(t, p.URL)
}

func TestBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := sainsburys.NewDatasource()

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Sainsburys, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

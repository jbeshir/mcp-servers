package shopify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/shopify"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func testConfig(baseURL string) shopify.Config {
	return shopify.Config{
		ID:      datasource.Hiyou,
		Name:    "HiYoU",
		BaseURL: baseURL,
	}
}

func TestSearchProducts(t *testing.T) {
	srv := testutil.JSONFixtureServer(t, "testdata/search.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	products, err := ds.SearchProducts(context.Background(), "rice")
	require.NoError(t, err)
	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "Golden Bowl Thai Hom Mali Rice 1kg", p.Name)
	assert.Equal(t, 2.85, p.Price)
	assert.Equal(t, "golden-bowl-thai-hom-mali-rice-1kg", p.ID)
	assert.Equal(t, datasource.Hiyou, p.Supermarket)
	assert.Equal(t, "GBP", p.Currency)
	assert.True(t, p.Available)
	assert.NotEmpty(t, p.ImageURL)
	assert.True(t, strings.HasPrefix(p.URL, srv.URL))

	// Second product: unavailable, image falls back to featured_image.
	p2 := products[1]
	assert.False(t, p2.Available)
	assert.NotEmpty(t, p2.ImageURL)
}

func TestGetProductDetails(t *testing.T) {
	srv := testutil.JSONFixtureServer(t, "testdata/product.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	p, err := ds.GetProductDetails(
		context.Background(), "golden-bowl-thai-hom-mali-rice-1kg",
	)
	require.NoError(t, err)

	assert.Equal(t, "Golden Bowl Thai Hom Mali Rice 1kg", p.Name)
	assert.Equal(t, 2.85, p.Price)
	assert.Equal(t, "1kg", p.Weight)
	assert.Equal(t, "golden-bowl-thai-hom-mali-rice-1kg", p.ID)
	assert.NotEmpty(t, p.ImageURL)
}

func TestBrowseCategories(t *testing.T) {
	srv := testutil.JSONFixtureServer(t, "testdata/collections.json")
	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.Len(t, categories, 2)

	assert.Equal(t, "Summer Sale", categories[0].Name)
	assert.Equal(t, datasource.Hiyou, categories[0].Supermarket)
	assert.Contains(t, categories[0].URL, "/collections/summer-sale")
}

func TestHTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}),
	)
	defer srv.Close()

	ds := shopify.NewDatasourceWithClient(testConfig(srv.URL), srv.Client())

	_, err := ds.SearchProducts(context.Background(), "rice")
	assert.Error(t, err)
}

package osp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestOcadoSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestMorrisonsSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()
	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	testutil.AssertSearchResults(t, products, "milk")
}

func TestOcadoProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()

	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name)
	assert.Positive(t, p.Price)
	assert.NotEmpty(t, p.URL)
}

func TestMorrisonsProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()

	products, err := ds.SearchProducts(context.Background(), "milk")
	require.NoError(t, err)
	require.NotEmpty(t, products, "no search results to look up")

	p, err := ds.GetProductDetails(context.Background(), products[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name)
	assert.Positive(t, p.Price)
	assert.NotEmpty(t, p.URL)
}

func TestOcadoBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewOcado()

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Ocado, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

func TestMorrisonsBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ds := osp.NewMorrisons()

	categories, err := ds.BrowseCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		assert.Equal(t, datasource.Morrisons, c.Supermarket)
		assert.NotEmpty(t, c.URL)
	}
}

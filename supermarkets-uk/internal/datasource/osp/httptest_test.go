package osp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestOcadoSearchProducts(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/ocado_search.html")
	ds := osp.NewOcado(osp.Config{BaseURL: srv.URL}, srv.Client())

	products, err := ds.SearchProducts(t.Context(), "milk")
	require.NoError(t, err)
	require.Len(t, products, 2)
	assert.Equal(t, "Cravendale Filtered Fresh Whole Milk Fresher for Longer", products[0].Name)
}

func TestOcadoGetProductDetails(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/ocado_product.html")
	ds := osp.NewOcado(osp.Config{BaseURL: srv.URL}, srv.Client())

	p, err := ds.GetProductDetails(t.Context(), "24577011")
	require.NoError(t, err)
	assert.Equal(t, "24577011", p.ID)
	assert.Contains(t, p.URL, "24577011")
	assert.NotEmpty(t, p.Description)
}

func TestOcadoBrowseCategories(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/ocado_categories.html")
	ds := osp.NewOcado(osp.Config{BaseURL: srv.URL}, srv.Client())

	categories, err := ds.BrowseCategories(t.Context())
	require.NoError(t, err)
	require.Len(t, categories, 2)
}

func TestMorrisonsSearchProducts(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/morrisons_search.html")
	ds := osp.NewMorrisons(osp.Config{BaseURL: srv.URL}, srv.Client())

	products, err := ds.SearchProducts(t.Context(), "cheesecake")
	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g", products[0].Name)
}

func TestMorrisonsGetProductDetails(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/morrisons_product.html")
	ds := osp.NewMorrisons(osp.Config{BaseURL: srv.URL}, srv.Client())

	p, err := ds.GetProductDetails(t.Context(), "12345")
	require.NoError(t, err)
	assert.Equal(t, "12345", p.ID)
	assert.Contains(t, p.URL, "12345")
	assert.NotEmpty(t, p.Description)
}

func TestMorrisonsBrowseCategories(t *testing.T) {
	srv := testutil.HTMLFixtureServer(t, "testdata/morrisons_categories.html")
	ds := osp.NewMorrisons(osp.Config{BaseURL: srv.URL}, srv.Client())

	categories, err := ds.BrowseCategories(t.Context())
	require.NoError(t, err)
	require.Len(t, categories, 3)
}

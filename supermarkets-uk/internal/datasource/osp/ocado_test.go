package osp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseOcadoSearchResults(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/ocado_search.html", osp.ParseOcadoSearchResults)

	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "Cravendale Filtered Fresh Whole Milk Fresher for Longer", p.Name)
	assert.Equal(t, 2.70, p.Price)
	assert.Equal(t, datasource.Ocado, p.Supermarket)
	assert.Equal(t, "24577011", p.ID)
}

func TestParseOcadoProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/ocado_product.html", osp.ParseOcadoProductPage)

	assert.Equal(t, "Cravendale Filtered Fresh Whole Milk Fresher for Longer", p.Name)
	assert.Equal(t, 2.70, p.Price)
}

func TestParseOcadoCategories(t *testing.T) {
	categories := testutil.ParseCategoryFile(t, "testdata/ocado_categories.html", osp.ParseOcadoCategories)

	require.Len(t, categories, 2)
	assert.Equal(t, "Fresh & Chilled Food", categories[0].Name)
	assert.Equal(t, datasource.Ocado, categories[0].Supermarket)
}

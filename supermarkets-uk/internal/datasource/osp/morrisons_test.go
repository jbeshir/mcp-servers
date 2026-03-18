package osp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseMorrisonsSearchResults(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/morrisons_search.html", osp.ParseMorrisonsSearchResults)

	require.Len(t, products, 1)

	p := products[0]
	assert.Equal(t, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g", p.Name)
	assert.InDelta(t, 2.80, p.Price, 0.001)
	assert.Equal(t, "115g", p.Weight)
	assert.Equal(t, datasource.Morrisons, p.Supermarket)
}

func TestParseMorrisonsSearchResults_Unavailable(t *testing.T) {
	products := testutil.ParseSearchFile(t, "testdata/morrisons_search_unavailable.html", osp.ParseMorrisonsSearchResults)

	require.Len(t, products, 2)

	assert.True(t, *products[0].Available, "in-stock product should be available")
	assert.Equal(t, "Yo! Sushi Chicken Katsu Dragon Rolls", products[0].Name)

	assert.False(t, *products[1].Available, "out-of-stock product should be unavailable")
	assert.Equal(t, "FTO Sushi Platter (40 Pieces) - Eat On Same Day", products[1].Name)
}

func TestParseMorrisonsCategories(t *testing.T) {
	categories := testutil.ParseCategoryFile(t, "testdata/morrisons_categories.html", osp.ParseMorrisonsCategories)

	require.Len(t, categories, 3)
	assert.Equal(t, "Fruit, Veg & Flowers", categories[0].Name)
	assert.Equal(t, datasource.Morrisons, categories[0].Supermarket)
}

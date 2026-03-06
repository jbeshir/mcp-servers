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
	assert.Equal(t, 2.80, p.Price)
	assert.Equal(t, "115g", p.Weight)
	assert.Equal(t, datasource.Morrisons, p.Supermarket)
}

func TestParseMorrisonsProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/morrisons_product.html", osp.ParseMorrisonsProductPage)

	assert.Equal(t, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g", p.Name)
	assert.Equal(t, 2.80, p.Price)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
}

func TestParseMorrisonsCategories(t *testing.T) {
	categories := testutil.ParseCategoryFile(t, "testdata/morrisons_categories.html", osp.ParseMorrisonsCategories)

	require.Len(t, categories, 3)
	assert.Equal(t, "Fruit, Veg & Flowers", categories[0].Name)
	assert.Equal(t, datasource.Morrisons, categories[0].Supermarket)
}

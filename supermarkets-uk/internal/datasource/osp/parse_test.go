package osp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseOcadoProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/ocado_product.html", parseOcadoProductPage)

	assert.Equal(t, "Cravendale Filtered Fresh Whole Milk Fresher for Longer", p.Name)
	assert.InDelta(t, 2.70, p.Price, 0.001)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
}

func TestParseMorrisonsProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/morrisons_product.html", parseMorrisonsProductPage)

	assert.Equal(t, "Mighty Slice Caramelised Biscuit High Protein Cheesecake 115g", p.Name)
	assert.InDelta(t, 2.80, p.Price, 0.001)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
}

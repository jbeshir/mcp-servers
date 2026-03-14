package waitrose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/waitrose_product.html", parseProductPage)

	assert.Equal(t, "Essential British Free Range Semi-Skimmed Milk 4 Pints", p.Name)
	assert.InDelta(t, 1.75, p.Price, 0.001)
	assert.Equal(t, "77p/litre", p.PricePerUnit)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
	assert.NotEmpty(t, p.Nutrition.PerPortion["Energy"])
}

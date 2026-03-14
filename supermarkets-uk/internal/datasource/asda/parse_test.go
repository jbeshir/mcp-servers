package asda

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/asda_product.html", parseProductPage)

	assert.Equal(t, "ASDA British Milk Semi Skimmed 4 Pints", p.Name)
	assert.InDelta(t, 1.65, p.Price, 0.001)
	assert.NotEmpty(t, p.PricePerUnit)
	assert.Equal(t, "4 pint", p.Weight)
	assert.NotEmpty(t, p.ImageURL)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
}

package tesco

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/testutil"
)

func TestParseProductPage(t *testing.T) {
	p := testutil.ParseProductFile(t, "testdata/tesco_product.html", ParseProductPage)

	assert.Equal(t, "Tesco British Semi Skimmed Milk 2.272L, 4 Pints", p.Name)
	assert.InDelta(t, 1.65, p.Price, 0.001)
	assert.Equal(t, "£0.73/litre", p.PricePerUnit)
	assert.NotEmpty(t, p.Description)
	assert.Contains(t, p.Ingredients, "Milk")
	require.NotNil(t, p.Nutrition)
	assert.NotEmpty(t, p.Nutrition.Per100g["Energy"])
	assert.NotEmpty(t, p.Nutrition.Per100g["Fat"])
	assert.NotEmpty(t, p.Nutrition.PerPortion["Energy"])
}

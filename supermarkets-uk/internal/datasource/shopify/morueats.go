package shopify

import "github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"

// NewMorueats creates a new Morueats datasource.
// Morueats is an Asian grocery covering Japanese, Chinese, Korean, and Thai products, running on Shopify.
func NewMorueats() *Datasource {
	return NewDatasource(Config{
		ID:          datasource.Morueats,
		Name:        "Morueats",
		Description: "Asian grocery covering Japanese, Chinese, Korean, and Thai products",
		BaseURL:     "https://morueats.com",
	})
}

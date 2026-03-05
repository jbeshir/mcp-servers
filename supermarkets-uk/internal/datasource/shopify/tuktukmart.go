package shopify

import "github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"

// NewTukTukMart creates a new Tuk Tuk Mart datasource.
// Tuk Tuk Mart is a Manchester-based Asian supermarket (Hang Won Hong's online store), running on Shopify.
func NewTukTukMart() *Datasource {
	return NewDatasource(Config{
		ID:          datasource.TukTukMart,
		Name:        "Tuk Tuk Mart",
		Description: "Manchester-based Asian supermarket (Hang Won Hong's online store)",
		BaseURL:     "https://tuktukmart.co.uk",
	})
}

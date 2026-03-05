package shopify

import "github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"

// NewHiyou creates a new HiYoU datasource.
// HiYoU is an Asian supermarket based in Newcastle, running on Shopify.
func NewHiyou() *Datasource {
	return NewDatasource(Config{
		ID:          datasource.Hiyou,
		Name:        "HiYoU",
		Description: "Asian supermarket based in Newcastle",
		BaseURL:     "https://hiyou.co",
	})
}

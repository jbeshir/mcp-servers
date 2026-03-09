package shopify

import (
	"net/http"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// NewHiyou creates a new HiYoU datasource.
// HiYoU is an Asian supermarket based in Newcastle, running on Shopify.
func NewHiyou(httpClient *http.Client) *Datasource {
	return NewDatasource(Config{
		ID:          datasource.Hiyou,
		Name:        "HiYoU",
		Description: "Asian supermarket based in Newcastle",
		BaseURL:     "https://hiyou.co",
	}, httpClient)
}

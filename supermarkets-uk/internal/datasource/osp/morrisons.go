package osp

import (
	"io"
	"net/http"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

// Shared HTML attribute and tag names used by the OSP selector configs.
const (
	attrDataTest = "data-test"

	tagDiv  = "div"
	tagSpan = "span"
)

const morrisonsBaseURL = "https://groceries.morrisons.com"

var morrisonsCfg = ospConfig{
	id:          datasource.Morrisons,
	name:        "Morrisons",
	description: "Major UK supermarket chain",
	baseURL:     morrisonsBaseURL,
	selectors: scraper.Config{
		ID:          datasource.Morrisons,
		BaseURL:     morrisonsBaseURL,
		Container:   scraper.ElemSel{Tag: tagDiv, Cls: "product-card-container"},
		CategorySel: scraper.ElemSel{Tag: "a", Att: attrDataTest, Val: "root-category-link"},
		SearchSel: scraper.ProductSelectors{
			Title:       scraper.ElemSel{Tag: "h3", Att: attrDataTest, Val: "fop-title"},
			Link:        scraper.ElemSel{Tag: "a", Att: attrDataTest, Val: "fop-product-link"},
			Price:       scraper.ElemSel{Tag: tagSpan, Att: attrDataTest, Val: "fop-price"},
			Unit:        scraper.ElemSel{Tag: tagSpan, Att: attrDataTest, Val: "fop-price-per-unit"},
			Promo:       scraper.ElemSel{Tag: tagSpan, Att: attrDataTest, Val: "fop-offer-text"},
			Image:       scraper.ElemSel{Tag: "img", Att: attrDataTest, Val: "fop-image"},
			Weight:      scraper.ElemSel{Tag: tagDiv, Att: attrDataTest, Val: "fop-size"},
			Unavailable: scraper.ElemSel{Tag: tagSpan, Att: attrDataTest, Val: "product-card-out-of-stock-badge"},
		},
		ProductSel: scraper.ProductSelectors{
			Title: scraper.ElemSel{Tag: "h1"},
			Price: scraper.ElemSel{Tag: tagDiv, Att: attrDataTest, Val: "price-container"},
			Promo: scraper.ElemSel{Tag: "a", Att: attrDataTest, Val: "offer-card-promotion"},
		},
	},
	sessionCheckQuery: scraper.ElemSel{Tag: "a", Att: attrDataTest, Val: "logout-button"},
	nutritionTableSel: scraper.ElemSel{Tag: "table", Cls: "nutrition"},
}

// NewMorrisons creates a new Morrisons datasource.
// Morrisons SSR HTML contains product data, so no browser is needed.
func NewMorrisons(cfg Config, httpClient *http.Client) datasource.AuthProductSource {
	resolved := morrisonsCfg
	if cfg.BaseURL != "" {
		resolved.baseURL = cfg.BaseURL
		resolved.selectors.BaseURL = cfg.BaseURL
	}
	return &ospDatasource{
		cfg:        resolved,
		httpClient: httpClient,
	}
}

// ParseMorrisonsSearchResults parses a Morrisons search results page.
func ParseMorrisonsSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, morrisonsCfg.selectors)
}

// parseMorrisonsProductPage parses a Morrisons product detail page.
// The returned Product does not have ID or URL set.
func parseMorrisonsProductPage(r io.Reader) (*datasource.Product, error) {
	return parseOSPProductPage(r, morrisonsCfg)
}

// ParseMorrisonsCategories parses a Morrisons browse/categories page.
func ParseMorrisonsCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, morrisonsCfg.selectors)
}

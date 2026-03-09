package osp

import (
	"io"
	"net/http"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
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
		Container:   scraper.ElemSel{Tag: "div", Cls: "product-card-container"},
		CategorySel: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "root-category-link"},
		SearchSel: scraper.ProductSelectors{
			Title:  scraper.ElemSel{Tag: "h3", Att: "data-test", Val: "fop-title"},
			Link:   scraper.ElemSel{Tag: "a", Att: "data-test", Val: "fop-product-link"},
			Price:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price"},
			Unit:   scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price-per-unit"},
			Promo:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-offer-text"},
			Image:  scraper.ElemSel{Tag: "img", Att: "data-test", Val: "fop-image"},
			Weight: scraper.ElemSel{Tag: "div", Att: "data-test", Val: "fop-size"},
		},
		ProductSel: scraper.ProductSelectors{
			Title: scraper.ElemSel{Tag: "h1"},
			Price: scraper.ElemSel{Tag: "div", Att: "data-test", Val: "price-container"},
			Promo: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "offer-card-promotion"},
		},
	},
	sessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "logout-button"},
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

// ParseMorrisonsProductPage parses a Morrisons product detail page.
func ParseMorrisonsProductPage(r io.Reader) (*datasource.Product, error) {
	return parseOSPProductPage(r, morrisonsCfg)
}

// ParseMorrisonsCategories parses a Morrisons browse/categories page.
func ParseMorrisonsCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, morrisonsCfg.selectors)
}

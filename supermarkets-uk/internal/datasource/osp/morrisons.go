package osp

import (
	"io"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const morrisonsBaseURL = "https://groceries.morrisons.com"

var morrisonsConfig = scraper.Config{
	ID:          datasource.Morrisons,
	Name:        "Morrisons",
	Description: "Major UK supermarket chain",
	BaseURL:     morrisonsBaseURL,
	SearchURL:   scraper.QuerySearchURL(morrisonsBaseURL+"/search", "q"),
	ProductURL:  scraper.ProductURLBuilder(morrisonsBaseURL + "/products/"),
	CategoryURL: morrisonsBaseURL + "/browse",
	Container:   scraper.ElemSel{Tag: "div", Cls: "product-card-container"},
	CategorySel: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "browse-link"},
	SearchSel: scraper.ProductSelectors{
		Title:  scraper.ElemSel{Tag: "h3", Att: "data-test", Val: "fop-title"},
		Link:   scraper.ElemSel{Tag: "a", Att: "data-test", Val: "fop-product-link"},
		Price:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price"},
		Unit:   scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price-per-unit"},
		Promo:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-offer-text"},
		Image:  scraper.ElemSel{Tag: "img", Att: "data-test", Val: "lazy-load-image"},
		Weight: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-catch-weight"},
	},
	SessionCheckURL:   morrisonsBaseURL + "/",
	SessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "logout-button"},
	ProductSel: scraper.ProductPageSelectors{
		Title:  scraper.ElemSel{Tag: "h1", Att: "data-test", Val: "product-title"},
		Price:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-price"},
		Unit:   scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-unit-price"},
		Promo:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-offer"},
		Image:  scraper.ElemSel{Tag: "img", Att: "data-test", Val: "product-image"},
		Weight: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-weight"},
	},
}

// NewMorrisons creates a new Morrisons datasource.
// Morrisons SSR HTML contains product data, so no browser is needed.
func NewMorrisons() *scraper.Scraper {
	return scraper.NewScraper(morrisonsConfig)
}

// ParseMorrisonsSearchResults parses a Morrisons search results page.
func ParseMorrisonsSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, morrisonsConfig)
}

// ParseMorrisonsProductPage parses a Morrisons product detail page.
func ParseMorrisonsProductPage(r io.Reader) (*datasource.Product, error) {
	return scraper.ParseProductPage(r, morrisonsConfig)
}

// ParseMorrisonsCategories parses a Morrisons browse/categories page.
func ParseMorrisonsCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, morrisonsConfig)
}

// Package tesco provides a Tesco supermarket datasource.
package tesco

import (
	"io"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const baseURL = "https://www.tesco.com"

var config = scraper.Config{
	ID:      datasource.Tesco,
	Name:    "Tesco",
	BaseURL: baseURL,
	SearchURL: scraper.QuerySearchURL(
		baseURL+"/groceries/en-GB/search", "query",
	),
	ProductURL:  scraper.ProductURLBuilder(baseURL + "/groceries/en-GB/products/"),
	CategoryURL: baseURL + "/groceries/en-GB/shop",
	Container:   scraper.ElemSel{Tag: "li", Att: "data-testid"},
	CategorySel: scraper.ElemSel{Tag: "a", Cls: "category-link"},
	SearchSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "a", Cls: "titleLink"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
	ProductSel: scraper.ProductPageSelectors{
		Title: scraper.ElemSel{Tag: "h1", Cls: "titleText"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
}

// NewDatasource creates a new Tesco datasource.
// Tesco requires a headless browser for JavaScript rendering.
func NewDatasource(browser *scraper.Browser) *scraper.BrowserScraper {
	return scraper.NewBrowserScraper(config, browser, `li[data-testid]`)
}

// ParseSearchResults parses a Tesco search results page.
func ParseSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, config)
}

// ParseProductPage parses a Tesco product detail page.
func ParseProductPage(r io.Reader) (*datasource.Product, error) {
	return scraper.ParseProductPage(r, config)
}

// ParseCategories parses a Tesco categories page.
func ParseCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, config)
}

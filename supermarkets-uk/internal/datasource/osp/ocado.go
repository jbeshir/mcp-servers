// Package osp provides datasources for supermarkets built on the Ocado Smart Platform.
package osp

import (
	"io"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const ocadoBaseURL = "https://www.ocado.com"

var ocadoConfig = scraper.Config{
	ID:          datasource.Ocado,
	Name:        "Ocado",
	BaseURL:     ocadoBaseURL,
	SearchURL:   scraper.QuerySearchURL(ocadoBaseURL+"/search", "q"),
	ProductURL:  scraper.ProductURLBuilder(ocadoBaseURL + "/products/"),
	CategoryURL: ocadoBaseURL + "/browse",
	Container:   scraper.ElemSel{Tag: "div", Cls: "product-card-container"},
	CategorySel: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "browse-link"},
	SearchSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "h3", Att: "data-test", Val: "fop-title"},
		Link:  scraper.ElemSel{Tag: "a", Att: "data-test", Val: "fop-product-link"},
		Price: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price"},
		Unit:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price-per-unit"},
		Promo: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-offer-text"},
		Image: scraper.ElemSel{Tag: "img", Att: "data-test", Val: "lazy-load-image"},
	},
	ProductSel: scraper.ProductPageSelectors{
		Title: scraper.ElemSel{Tag: "h1", Att: "data-test", Val: "product-title"},
		Price: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-price"},
		Unit:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-unit-price"},
		Promo: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-offer"},
		Image: scraper.ElemSel{Tag: "img", Att: "data-test", Val: "product-image"},
	},
}

// NewOcado creates a new Ocado datasource.
// Ocado SSR HTML contains product data, so no browser is needed.
func NewOcado() *scraper.Scraper {
	return scraper.NewScraper(ocadoConfig)
}

// ParseOcadoSearchResults parses an Ocado search results page.
func ParseOcadoSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, ocadoConfig)
}

// ParseOcadoProductPage parses an Ocado product detail page.
func ParseOcadoProductPage(r io.Reader) (*datasource.Product, error) {
	return scraper.ParseProductPage(r, ocadoConfig)
}

// ParseOcadoCategories parses an Ocado browse/categories page.
func ParseOcadoCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, ocadoConfig)
}

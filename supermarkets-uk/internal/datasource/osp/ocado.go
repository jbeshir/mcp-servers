// Package osp provides datasources for supermarkets built on the Ocado Smart Platform.
package osp

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

// ospConfig holds the full configuration for an OSP-based supermarket.
type ospConfig struct {
	id                datasource.SupermarketID
	name              string
	description       string
	searchURL         func(query string) string
	productURL        func(id string) string
	categoryURL       string
	selectors         scraper.Config
	sessionCheckURL   string
	sessionCheckQuery scraper.ElemSel
	nutritionTableSel scraper.ElemSel
}

// ospDatasource implements datasource.AuthDatasource for an OSP-based supermarket.
type ospDatasource struct {
	cfg        ospConfig
	cookies    []*http.Cookie
	httpClient *http.Client
}

func (d *ospDatasource) ID() datasource.SupermarketID      { return d.cfg.id }
func (d *ospDatasource) Name() string                      { return d.cfg.name }
func (d *ospDatasource) Description() string               { return d.cfg.description }
func (d *ospDatasource) SetCookies(cookies []*http.Cookie) { d.cookies = cookies }

func (d *ospDatasource) CheckSession(ctx context.Context) bool {
	if len(d.cookies) == 0 {
		return true
	}
	body, err := scraper.FetchHTML(ctx, d.cfg.sessionCheckURL, d.cookies, d.httpClient)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.HTMLHasElement(body, d.cfg.sessionCheckQuery)
}

func (d *ospDatasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	body, err := scraper.FetchHTML(ctx, d.cfg.searchURL(query), d.cookies, d.httpClient)
	if err != nil {
		return nil, fmt.Errorf("%s search fetch: %w", d.cfg.id, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.ParseSearchResults(body, d.cfg.selectors)
}

func (d *ospDatasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := scraper.FetchHTML(ctx, d.cfg.productURL(productID), d.cookies, d.httpClient)
	if err != nil {
		return nil, fmt.Errorf("%s product fetch: %w", d.cfg.id, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := parseOSPProductPage(body, d.cfg)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = d.cfg.productURL(productID)
	return p, nil
}

func (d *ospDatasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := scraper.FetchHTML(ctx, d.cfg.categoryURL, d.cookies, d.httpClient)
	if err != nil {
		return nil, fmt.Errorf("%s categories fetch: %w", d.cfg.id, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.ParseCategories(body, d.cfg.selectors)
}

// parseOSPProductPage parses an OSP product detail page.
// OSP pages use h2 headings to label sections (e.g. "Product Information",
// "Ingredients"), so description and ingredients are extracted by heading text
// rather than data-test selectors.
func parseOSPProductPage(r io.Reader, cfg ospConfig) (*datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%s: parse product HTML: %w", cfg.id, err)
	}

	p := scraper.ParseProductFields(doc, cfg.selectors.ProductSel, cfg.id)
	p.Description = sectionContent(doc, "Product Information")
	p.Ingredients = sectionContent(doc, "Ingredients")
	table := scraper.FindNutritionTable(doc, cfg.nutritionTableSel)
	p.Nutrition = scraper.ParseNutritionTable(table)
	return p, nil
}

// sectionContent finds an h2 whose text matches heading and returns the text
// content of the next sibling element. OSP product pages use this pattern for
// all detail sections.
func sectionContent(doc *html.Node, heading string) string {
	var result string
	scraper.WalkTree(doc, func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type != html.ElementNode || n.Data != "h2" {
			return
		}
		if scraper.TextContent(n) != heading {
			return
		}
		// Return text of the next sibling element.
		for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
			if sib.Type == html.ElementNode {
				result = scraper.TextContent(sib)
				return
			}
		}
	})
	return result
}

// --- Ocado ---

const ocadoBaseURL = "https://www.ocado.com"

var ocadoCfg = ospConfig{
	id:          datasource.Ocado,
	name:        "Ocado",
	description: "Online-only UK supermarket and grocery delivery service",
	searchURL:   scraper.QuerySearchURL(ocadoBaseURL+"/search", "q"),
	productURL:  scraper.ProductURLBuilder(ocadoBaseURL + "/products/"),
	categoryURL: ocadoBaseURL + "/categories",
	selectors: scraper.Config{
		ID:          datasource.Ocado,
		BaseURL:     ocadoBaseURL,
		Container:   scraper.ElemSel{Tag: "div", Cls: "product-card-container"},
		CategorySel: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "root-category-link"},
		SearchSel: scraper.ProductSelectors{
			Title: scraper.ElemSel{Tag: "h3", Att: "data-test", Val: "fop-title"},
			Link:  scraper.ElemSel{Tag: "a", Att: "data-test", Val: "fop-product-link"},
			Price: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price"},
			Unit:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price-per-unit"},
			Promo: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-offer-text"},
			Image: scraper.ElemSel{Tag: "img", Att: "data-test", Val: "fop-image"},
		},
		ProductSel: scraper.ProductSelectors{
			Title: scraper.ElemSel{Tag: "h1"},
			Price: scraper.ElemSel{Tag: "div", Att: "data-test", Val: "price-container"},
			Promo: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "offer-card-promotion"},
		},
	},
	sessionCheckURL:   ocadoBaseURL + "/",
	sessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "logout-button"},
	nutritionTableSel: scraper.ElemSel{Tag: "table", Cls: "nutrition"},
}

// NewOcado creates a new Ocado datasource.
// Ocado SSR HTML contains product data, so no browser is needed.
func NewOcado() datasource.AuthDatasource {
	return &ospDatasource{
		cfg:        ocadoCfg,
		httpClient: &http.Client{},
	}
}

// ParseOcadoSearchResults parses an Ocado search results page.
func ParseOcadoSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, ocadoCfg.selectors)
}

// ParseOcadoProductPage parses an Ocado product detail page.
func ParseOcadoProductPage(r io.Reader) (*datasource.Product, error) {
	return parseOSPProductPage(r, ocadoCfg)
}

// ParseOcadoCategories parses an Ocado browse/categories page.
func ParseOcadoCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, ocadoCfg.selectors)
}

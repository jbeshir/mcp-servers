// Package osp provides datasources for supermarkets built on the Ocado Smart Platform.
package osp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

// ospConfig holds the full configuration for an OSP-based supermarket.
type ospConfig struct {
	id                datasource.SupermarketID
	name              string
	description       string
	baseURL           string
	selectors         scraper.Config
	sessionCheckQuery scraper.ElemSel
	nutritionTableSel scraper.ElemSel
}

// Config holds optional overrides for an OSP datasource.
// Zero values use the built-in defaults.
type Config struct {
	BaseURL string
}

// ospDatasource implements datasource.AuthProductSource for an OSP-based supermarket.
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
	body, err := scraper.FetchHTML(ctx, d.cfg.baseURL+"/", d.cookies, d.httpClient)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.HTMLHasElement(body, d.cfg.sessionCheckQuery)
}

func (d *ospDatasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	searchURL := d.cfg.baseURL + "/search?" + url.Values{"q": {query}}.Encode()
	body, err := scraper.FetchHTML(ctx, searchURL, d.cookies, d.httpClient)
	if err != nil {
		return nil, fmt.Errorf("%s search fetch: %w", d.cfg.id, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.ParseSearchResults(body, d.cfg.selectors)
}

func (d *ospDatasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	productURL := d.cfg.baseURL + "/products/" + url.PathEscape(productID)
	body, err := scraper.FetchHTML(ctx, productURL, d.cookies, d.httpClient)
	if err != nil {
		return nil, fmt.Errorf("%s product fetch: %w", d.cfg.id, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := parseOSPProductPage(body, d.cfg)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = productURL
	return p, nil
}

func (d *ospDatasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := scraper.FetchHTML(ctx, d.cfg.baseURL+"/categories", d.cookies, d.httpClient)
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
	p.Description = scraper.SectionContent(doc, "h2", "Product Information")
	p.Ingredients = scraper.SectionContent(doc, "h2", "Ingredients")
	table := scraper.FindNutritionTable(doc, cfg.nutritionTableSel)
	p.Nutrition = scraper.ParseNutritionTable(table)

	// Check for OOS badge on the product page.
	if cfg.selectors.ProductSel.Unavailable != (scraper.ElemSel{}) {
		if el := scraper.FindElement(doc, cfg.selectors.ProductSel.Unavailable); el != nil {
			text := strings.ToLower(scraper.TextContent(el))
			if strings.Contains(text, "out of stock") ||
				strings.Contains(text, "unavailable") {
				p.Available = datasource.BoolPtr(false)
			}
		}
	}

	return p, nil
}

// --- Ocado ---

const ocadoBaseURL = "https://www.ocado.com"

var ocadoCfg = ospConfig{
	id:          datasource.Ocado,
	name:        "Ocado",
	description: "Online-only UK supermarket and grocery delivery service",
	baseURL:     ocadoBaseURL,
	selectors: scraper.Config{
		ID:          datasource.Ocado,
		BaseURL:     ocadoBaseURL,
		Container:   scraper.ElemSel{Tag: "div", Cls: "product-card-container"},
		CategorySel: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "root-category-link"},
		SearchSel: scraper.ProductSelectors{
			Title:       scraper.ElemSel{Tag: "h3", Att: "data-test", Val: "fop-title"},
			Link:        scraper.ElemSel{Tag: "a", Att: "data-test", Val: "fop-product-link"},
			Price:       scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price"},
			Unit:        scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-price-per-unit"},
			Promo:       scraper.ElemSel{Tag: "span", Att: "data-test", Val: "fop-offer-text"},
			Image:       scraper.ElemSel{Tag: "img", Att: "data-test", Val: "fop-image"},
			Unavailable: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-card-out-of-stock-badge"},
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

// NewOcado creates a new Ocado datasource.
// Ocado SSR HTML contains product data, so no browser is needed.
func NewOcado(cfg Config, httpClient *http.Client) datasource.AuthProductSource {
	resolved := ocadoCfg
	if cfg.BaseURL != "" {
		resolved.baseURL = cfg.BaseURL
		resolved.selectors.BaseURL = cfg.BaseURL
	}
	return &ospDatasource{
		cfg:        resolved,
		httpClient: httpClient,
	}
}

// ParseOcadoSearchResults parses an Ocado search results page.
func ParseOcadoSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, ocadoCfg.selectors)
}

// parseOcadoProductPage parses an Ocado product detail page.
// The returned Product does not have ID or URL set.
func parseOcadoProductPage(r io.Reader) (*datasource.Product, error) {
	return parseOSPProductPage(r, ocadoCfg)
}

// ParseOcadoCategories parses an Ocado browse/categories page.
func ParseOcadoCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, ocadoCfg.selectors)
}

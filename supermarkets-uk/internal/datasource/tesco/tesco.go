// Package tesco provides a Tesco supermarket datasource.
package tesco

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const baseURL = "https://www.tesco.com"

// waitSelector is the CSS selector to wait for before capturing search HTML.
const waitSelector = `li[data-testid]`

var (
	categoryURL       = baseURL + "/groceries/en-GB/search?query=a"
	sessionCheckURL   = baseURL + "/"
	sessionCheckQuery = scraper.ElemSel{Tag: "a", Att: "id", Val: "app-bar-sign-out"}
	nutritionTableSel = scraper.ElemSel{Tag: "table", Cls: "product__info-table"}
)

var selectors = scraper.Config{
	ID:          datasource.Tesco,
	BaseURL:     baseURL,
	Container:   scraper.ElemSel{Tag: "li", Att: "data-testid"},
	CategorySel: scraper.ElemSel{Tag: "a", Cls: "ddsweb-local-navigation__submenu-link"},
	SearchSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "a", Cls: "titleLink"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
	ProductSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "h1", Att: "data-auto", Val: "pdp-product-title"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
}

// Datasource implements datasource.AuthProductSource for Tesco using a headless browser.
type Datasource struct {
	browser    *scraper.Browser
	httpClient *http.Client
	cookies    []*http.Cookie
}

// NewDatasource creates a new Tesco datasource.
// Tesco requires a headless browser for JavaScript rendering.
func NewDatasource(browser *scraper.Browser, httpClient *http.Client) *Datasource {
	return &Datasource{browser: browser, httpClient: httpClient}
}

func (d *Datasource) SetCookies(cookies []*http.Cookie) { d.cookies = cookies }

func (d *Datasource) ID() datasource.SupermarketID { return datasource.Tesco }
func (d *Datasource) Name() string                 { return "Tesco" }
func (d *Datasource) Description() string          { return "The UK's largest supermarket chain" }

func (d *Datasource) CheckSession(ctx context.Context) bool {
	if len(d.cookies) == 0 {
		return true
	}
	body, err := d.browser.Fetch(ctx, sessionCheckURL, d.cookies)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.HTMLHasElement(body, sessionCheckQuery)
}

// SearchProducts searches for products using the search wait selector.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	searchURL := baseURL + "/groceries/en-GB/search?query=" + url.QueryEscape(query)
	body, err := d.browser.Fetch(ctx, searchURL, d.cookies, waitSelector)
	if err != nil {
		return nil, fmt.Errorf("tesco search fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.ParseSearchResults(body, selectors)
}

// GetProductDetails fetches product details.
func (d *Datasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := d.browser.Fetch(ctx, baseURL+"/groceries/en-GB/products/"+url.PathEscape(productID), d.cookies, `h1`)
	if err != nil {
		return nil, fmt.Errorf("tesco product fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := parseProductPage(body)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = baseURL + "/groceries/en-GB/products/" + url.PathEscape(productID)
	return p, nil
}

// BrowseCategories returns top-level grocery categories from the navigation
// on a search page.
func (d *Datasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := d.browser.Fetch(ctx, categoryURL, d.cookies, waitSelector)
	if err != nil {
		return nil, fmt.Errorf("tesco categories fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.ParseCategories(body, selectors)
}

// ParseSearchResults parses a Tesco search results page.
func ParseSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, selectors)
}

// parseProductPage parses a Tesco product detail page.
// The returned Product does not have ID or URL set.
func parseProductPage(r io.Reader) (*datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("tesco: parse product HTML: %w", err)
	}
	p := scraper.ParseProductFields(doc, selectors.ProductSel, datasource.Tesco)
	p.Description = scraper.SectionContent(doc, "h3", "Description")
	p.Ingredients = scraper.SectionContent(doc, "h3", "Ingredients")
	table := scraper.FindNutritionTable(doc, nutritionTableSel)
	p.Nutrition = scraper.ParseNutritionTable(table)
	return p, nil
}

// ParseCategories parses a Tesco categories page.
func ParseCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, selectors)
}

// Package tesco provides a Tesco supermarket datasource.
package tesco

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const baseURL = "https://www.tesco.com"

// waitSelector is the CSS selector to wait for before capturing search HTML.
const waitSelector = `li[data-testid]`

var config = scraper.Config{
	ID:          datasource.Tesco,
	Name:        "Tesco",
	Description: "The UK's largest supermarket chain",
	BaseURL:     baseURL,
	SearchURL: scraper.QuerySearchURL(
		baseURL+"/groceries/en-GB/search", "query",
	),
	ProductURL:  scraper.ProductURLBuilder(baseURL + "/groceries/en-GB/products/"),
	CategoryURL: baseURL + "/groceries/en-GB/search?query=a",
	Container:   scraper.ElemSel{Tag: "li", Att: "data-testid"},
	CategorySel: scraper.ElemSel{Tag: "a", Cls: "ddsweb-local-navigation__submenu-link"},
	SearchSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "a", Cls: "titleLink"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
	SessionCheckURL:   baseURL + "/",
	SessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "id", Val: "app-bar-sign-out"},
	ProductSel: scraper.ProductSelectors{
		Title: scraper.ElemSel{Tag: "h1", Att: "data-auto", Val: "pdp-product-title"},
		Price: scraper.ElemSel{Tag: "p", Cls: "priceText"},
		Unit:  scraper.ElemSel{Tag: "p", Cls: "subtext"},
		Promo: scraper.ElemSel{Tag: "span", Cls: "promotionText"},
		Image: scraper.ElemSel{Tag: "img", Cls: "baseImage"},
	},
}

// Datasource wraps a BrowserScraper but overrides GetProductDetails and
// BrowseCategories to avoid using the search wait selector on non-search pages.
type Datasource struct {
	inner *scraper.BrowserScraper
}

// NewDatasource creates a new Tesco datasource.
// Tesco requires a headless browser for JavaScript rendering.
func NewDatasource(browser *scraper.Browser) *Datasource {
	return &Datasource{
		inner: scraper.NewBrowserScraper(config, browser, waitSelector),
	}
}

// SetCookies sets session cookies.
func (d *Datasource) SetCookies(cookies []*http.Cookie) { d.inner.SetCookies(cookies) }

// ID returns the supermarket identifier.
func (d *Datasource) ID() datasource.SupermarketID { return d.inner.ID() }

// Name returns the human-readable name.
func (d *Datasource) Name() string { return d.inner.Name() }

// Description returns a short description.
func (d *Datasource) Description() string { return d.inner.Description() }

// CheckSession validates the session.
func (d *Datasource) CheckSession(ctx context.Context) bool { return d.inner.CheckSession(ctx) }

// SearchProducts searches for products using the search wait selector.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	return d.inner.SearchProducts(ctx, query)
}

// GetProductDetails fetches product details.
func (d *Datasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := d.inner.FetchPage(ctx, config.ProductURL(productID), `h1`)
	if err != nil {
		return nil, fmt.Errorf("tesco product fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := scraper.ParseProductPage(body, config)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = config.ProductURL(productID)
	return p, nil
}

// BrowseCategories returns top-level grocery categories from the navigation
// on a search page.
func (d *Datasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := d.inner.FetchPage(ctx, config.CategoryURL, waitSelector)
	if err != nil {
		return nil, fmt.Errorf("tesco categories fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return scraper.ParseCategories(body, config)
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

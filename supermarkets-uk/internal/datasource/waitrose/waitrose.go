// Package waitrose provides a Waitrose supermarket datasource.
package waitrose

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const baseURL = "https://www.waitrose.com"

// categoryHrefPrefix is the path prefix for top-level grocery category links.
const categoryHrefPrefix = "/ecom/shop/browse/groceries/"

// waitSelector is the CSS selector to wait for before capturing search HTML.
const waitSelector = `article[data-testid="product-pod"]`

var config = scraper.Config{
	ID:          datasource.Waitrose,
	Name:        "Waitrose",
	Description: "Premium UK supermarket chain",
	BaseURL:     baseURL,
	SearchURL: scraper.QuerySearchURL(
		baseURL+"/ecom/shop/search", "searchTerm",
	),
	ProductURL: func(id string) string {
		return baseURL + "/ecom/products/" + id
	},
	CategoryURL: baseURL + "/ecom/shop/browse",
	Container:   scraper.ElemSel{Tag: "article", Att: "data-testid", Val: "product-pod"},
	CategorySel: scraper.ElemSel{Tag: "a", Att: "data-testid", Val: "browse-category-link"},
	SearchSel: scraper.ProductSelectors{
		Title:  scraper.ElemSel{Tag: "span", Cls: "name___"},
		Link:   scraper.ElemSel{Tag: "a", Cls: "nameLink"},
		Price:  scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-pod-price"},
		Unit:   scraper.ElemSel{Tag: "span", Cls: "pricePerUnit"},
		Promo:  scraper.ElemSel{Tag: "span", Att: "data-testid", Val: "description"},
		Image:  scraper.ElemSel{Tag: "img"},
		Weight: scraper.ElemSel{Tag: "span", Att: "data-testid", Val: "product-size"},
	},
	SessionCheckURL:   baseURL + "/",
	SessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "data-test", Val: "signOut"},
	ProductSel: scraper.ProductPageSelectors{
		Title: scraper.ElemSel{Tag: "span", Att: "data-testid", Val: "product-name"},
		Price: scraper.ElemSel{Tag: "span", Att: "data-test", Val: "product-pod-price"},
		Unit:  scraper.ElemSel{Tag: "span", Cls: "ProductPricing_pricePerUnit"},
	},
}

// Datasource wraps a BrowserScraper with Waitrose-specific overrides
// for search (pence price parsing), product details, and categories.
type Datasource struct {
	inner *scraper.BrowserScraper
}

// NewDatasource creates a new Waitrose datasource.
// Waitrose requires a headless browser for JavaScript rendering.
func NewDatasource(browser *scraper.Browser) *Datasource {
	return &Datasource{
		inner: scraper.NewBrowserScraper(config, browser, `article[data-testid="product-pod"]`),
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

// SearchProducts searches for products with Waitrose-specific price parsing.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	body, err := d.inner.FetchPage(ctx, config.SearchURL(query), waitSelector)
	if err != nil {
		return nil, fmt.Errorf("waitrose search fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("waitrose: parse HTML: %w", err)
	}

	return parseProducts(doc)
}

// GetProductDetails fetches product details.
func (d *Datasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := d.inner.FetchPage(ctx, config.ProductURL(productID), `h1`)
	if err != nil {
		return nil, fmt.Errorf("waitrose product fetch: %w", err)
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

// BrowseCategories returns top-level grocery categories using custom parsing.
func (d *Datasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := d.inner.FetchPage(ctx, config.CategoryURL)
	if err != nil {
		return nil, fmt.Errorf("waitrose categories fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseCategories(body)
}

// ParseSearchResults parses a Waitrose search results page with
// Waitrose-specific price parsing (handles pence-only prices like "95p").
func ParseSearchResults(r io.Reader) ([]datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("waitrose: parse HTML: %w", err)
	}

	return parseProducts(doc)
}

// ParseProductPage parses a Waitrose product detail page.
func ParseProductPage(r io.Reader) (*datasource.Product, error) {
	return scraper.ParseProductPage(r, config)
}

// parseProducts extracts products from a parsed HTML document, applying
// Waitrose-specific pence price parsing.
func parseProducts(doc *html.Node) ([]datasource.Product, error) {
	var products []datasource.Product
	scraper.WalkTree(doc, func(n *html.Node) {
		if !config.Container.Matches(n) {
			return
		}
		p, ok := scraper.ExtractProduct(n, config.SearchSel, config.BaseURL, config.ID)
		if !ok {
			return
		}

		// Waitrose product URLs have two path segments after /ecom/products/:
		// a slug and a numeric ID (e.g. "essential-milk/053457-26759-26760").
		// Override the ID to include both so ProductURL builds a valid URL.
		if p.URL != "" {
			p.ID = productIDFromURL(p.URL)
		}

		// Override price with Waitrose-aware parsing that handles pence.
		if elem := scraper.FindElement(n, config.SearchSel.Price); elem != nil {
			p.Price = parseWaitrosePrice(scraper.TextContent(elem))
		}

		products = append(products, p)
	})

	if len(products) == 0 {
		return nil, fmt.Errorf(
			"waitrose: no products found in HTML (page may require JavaScript rendering)",
		)
	}
	return products, nil
}

// parseWaitrosePrice parses a price string, handling both "£1.50" and
// pence-only formats like "95p" or "Item price95p".
func parseWaitrosePrice(s string) float64 {
	if f := scraper.ParsePrice(s); f != 0 {
		return f
	}
	// Fall back to pence-only: walk backwards from the last "p" to extract digits.
	if i := strings.LastIndex(s, "p"); i > 0 {
		j := i - 1
		for j >= 0 && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
			j--
		}
		candidate := s[j+1 : i]
		if f, err := strconv.ParseFloat(candidate, 64); err == nil {
			return f / 100
		}
	}
	return 0
}

// productIDFromURL extracts the last two path segments from a Waitrose product
// URL, e.g. "https://…/ecom/products/slug-name/053457-26759-26760" → "slug-name/053457-26759-26760".
func productIDFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// ParseCategories parses a Waitrose categories page by finding <a> elements
// whose href starts with the groceries browse prefix and filtering to
// top-level categories only (one path segment after /groceries/).
func ParseCategories(r io.Reader) ([]datasource.Category, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("waitrose: parse categories HTML: %w", err)
	}

	seen := make(map[string]bool)
	var categories []datasource.Category
	scraper.WalkTree(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		href := scraper.GetAttr(n, "href")
		if !strings.HasPrefix(href, categoryHrefPrefix) {
			return
		}

		// Filter to top-level categories: exactly one segment after /groceries/.
		suffix := strings.TrimPrefix(href, categoryHrefPrefix)
		if suffix == "" || strings.Contains(suffix, "/") {
			return
		}

		// Deduplicate by href.
		if seen[href] {
			return
		}
		seen[href] = true

		name := scraper.TextContent(n)
		if name == "" {
			return
		}

		id := scraper.GetAttr(n, "id")
		categories = append(categories, datasource.Category{
			ID:          id,
			Name:        name,
			URL:         baseURL + href,
			Supermarket: datasource.Waitrose,
		})
	})

	return categories, nil
}

// Package asda provides an Asda supermarket datasource.
// Search uses the Algolia API directly; categories and product details
// use browser-rendered HTML.
package asda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

const (
	baseURL = "https://www.asda.com"

	algoliaAppID  = "8I6WSKCCNV"
	algoliaAPIKey = "03e4272048dd17f771da37b57ff8a75e" //nolint:gosec // Public search-only API key, not a secret.
	algoliaIndex  = "ASDA_PRODUCTS"
	algoliaURL    = "https://" + algoliaAppID + "-dsn.algolia.net/1/indexes/" + algoliaIndex + "/query"

	imageURLBase = "https://asdagroceries.scene7.com/is/image/asdagroceries/"
)

// browserConfig retains browser-based settings for categories and product pages.
var browserConfig = scraper.Config{
	ID:          datasource.Asda,
	Name:        "Asda",
	Description: "One of the UK's largest supermarket chains",
	BaseURL:     baseURL,
	SearchURL: func(query string) string {
		return baseURL + "/groceries/search/" + url.PathEscape(query)
	},
	ProductURL:  scraper.ProductURLBuilder(baseURL + "/groceries/product/"),
	CategoryURL: baseURL + "/groceries",
	Container:   scraper.ElemSel{Tag: "div", Att: "data-locator", Val: "single_product_wrapper"},
	CategorySel: scraper.ElemSel{Tag: "a", Att: "data-group", Val: "true"},
	SearchSel: scraper.ProductSelectors{
		Title:  scraper.ElemSel{Tag: "a", Att: "data-locator", Val: "txt-product-name"},
		Price:  scraper.ElemSel{Tag: "p", Att: "data-locator", Val: "txt-product-price"},
		Unit:   scraper.ElemSel{Tag: "p", Att: "data-locator", Val: "txt-product-price-per-uom"},
		Promo:  scraper.ElemSel{Tag: "a", Att: "data-locator", Val: "lnk-product-offer"},
		Image:  scraper.ElemSel{Tag: "img", Att: "data-locator", Val: "img-product-image"},
		Weight: scraper.ElemSel{Tag: "p", Att: "data-locator", Val: "txt-product-weight"},
	},
	SessionCheckURL:   baseURL + "/",
	SessionCheckQuery: scraper.ElemSel{Tag: "a", Att: "data-locator", Val: "btn-sign-in"},
	ProductSel: scraper.ProductPageSelectors{
		Title:  scraper.ElemSel{Tag: "h1", Att: "data-testid", Val: "txt-pdp-product-name"},
		Price:  scraper.ElemSel{Tag: "div", Att: "data-testid", Val: "txt-pdp-product-price"},
		Unit:   scraper.ElemSel{Tag: "div", Att: "data-testid", Val: "txt-pdp-product-price-per-kg"},
		Image:  scraper.ElemSel{Tag: "img", Att: "data-testid", Val: "img"},
		Weight: scraper.ElemSel{Tag: "p", Att: "data-testid", Val: "txt-pdp-weight-size"},
	},
}

// algoliaHit represents a single product hit from the Algolia search response.
type algoliaHit struct {
	ID       string                  `json:"ID"`
	ObjectID string                  `json:"objectID"`
	Name     string                  `json:"NAME"`
	ImageID  string                  `json:"IMAGE_ID"`
	PackSize string                  `json:"PACK_SIZE"`
	Prices   map[string]algoliaPrice `json:"PRICES"`
}

// algoliaPrice holds pricing for a region.
type algoliaPrice struct {
	Price                float64 `json:"PRICE"`
	Offer                string  `json:"OFFER"`
	PricePerUOMFormatted string  `json:"PRICEPERUOMFORMATTED"`
}

// algoliaResponse is the top-level Algolia search response.
type algoliaResponse struct {
	Hits []algoliaHit `json:"hits"`
}

// Datasource uses the Algolia API for search and a headless browser
// for categories and product detail pages.
type Datasource struct {
	browser    *scraper.Browser
	cookies    []*http.Cookie
	httpClient *http.Client
}

// NewDatasource creates a new Asda datasource.
func NewDatasource(browser *scraper.Browser) *Datasource {
	return &Datasource{
		browser:    browser,
		httpClient: &http.Client{},
	}
}

// SetCookies sets session cookies.
func (d *Datasource) SetCookies(cookies []*http.Cookie) { d.cookies = cookies }

// ID returns the supermarket identifier.
func (d *Datasource) ID() datasource.SupermarketID { return datasource.Asda }

// Name returns the human-readable name.
func (d *Datasource) Name() string { return "Asda" }

// Description returns a short description of the supermarket.
func (d *Datasource) Description() string { return "One of the UK's largest supermarket chains" }

// CheckSession validates whether cached cookies represent a valid session.
func (d *Datasource) CheckSession(ctx context.Context) bool {
	if len(d.cookies) == 0 || browserConfig.SessionCheckURL == "" {
		return true
	}
	body, err := d.browser.Fetch(ctx, browserConfig.SessionCheckURL, d.cookies)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return scraper.HTMLHasElement(body, browserConfig.SessionCheckQuery)
}

// SearchProducts searches for products via the Algolia API.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	payload, err := json.Marshal(map[string]string{
		"params": "query=" + url.QueryEscape(query) + "&hitsPerPage=60",
	})
	if err != nil {
		return nil, fmt.Errorf("asda: marshal algolia request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, algoliaURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("asda: create algolia request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Algolia-Application-Id", algoliaAppID)
	req.Header.Set("X-Algolia-API-Key", algoliaAPIKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asda: algolia request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort close.

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asda: algolia HTTP %d", resp.StatusCode)
	}

	return ParseAlgoliaResults(resp.Body)
}

// GetProductDetails fetches product details via the browser.
func (d *Datasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	waitSel := `[data-testid="txt-pdp-product-name"]`
	body, err := d.browser.Fetch(ctx, browserConfig.ProductURL(productID), d.cookies, waitSel)
	if err != nil {
		return nil, fmt.Errorf("asda product fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := scraper.ParseProductPage(body, browserConfig)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = browserConfig.ProductURL(productID)
	return p, nil
}

// BrowseCategories returns the top-level grocery categories via the browser.
func (d *Datasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	waitSel := `[data-group="true"]`
	body, err := d.browser.Fetch(ctx, browserConfig.CategoryURL, d.cookies, waitSel)
	if err != nil {
		return nil, fmt.Errorf("asda categories fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseCategories(body)
}

// ParseAlgoliaResults parses an Algolia search response JSON into products.
func ParseAlgoliaResults(r io.Reader) ([]datasource.Product, error) {
	var resp algoliaResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("asda: decode algolia response: %w", err)
	}

	products := make([]datasource.Product, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		products = append(products, convertHit(hit))
	}
	return products, nil
}

// ParseSearchResults parses an Asda search results HTML page (legacy).
func ParseSearchResults(r io.Reader) ([]datasource.Product, error) {
	return scraper.ParseSearchResults(r, browserConfig)
}

// ParseProductPage parses an Asda product detail page.
func ParseProductPage(r io.Reader) (*datasource.Product, error) {
	return scraper.ParseProductPage(r, browserConfig)
}

// ParseCategories parses an Asda categories page.
func ParseCategories(r io.Reader) ([]datasource.Category, error) {
	return scraper.ParseCategories(r, browserConfig)
}

func convertHit(hit algoliaHit) datasource.Product {
	p := datasource.Product{
		ID:          hit.ObjectID,
		Supermarket: datasource.Asda,
		Name:        hit.Name,
		Currency:    "GBP",
		Available:   true,
		URL:         baseURL + "/groceries/product/" + hit.ObjectID,
	}

	if hit.ImageID != "" {
		p.ImageURL = imageURLBase + hit.ImageID
	}
	if hit.PackSize != "" {
		p.Weight = hit.PackSize
	}

	// Use EN (England) region pricing as default.
	if price, ok := hit.Prices["EN"]; ok {
		p.Price = price.Price
		if price.PricePerUOMFormatted != "" {
			p.PricePerUnit = price.PricePerUOMFormatted
		}
		if price.Offer != "" && !strings.EqualFold(price.Offer, "none") {
			p.Promotion = price.Offer
		}
	}

	return p
}

// Package sainsburys provides a Sainsbury's supermarket datasource using the JSON API.
package sainsburys

import (
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
	baseURL = "https://www.sainsburys.co.uk"
	apiBase = baseURL + "/groceries-api/gol-services"
)

// apiProduct is the JSON structure returned by the Sainsbury's API.
type apiProduct struct {
	ProductUID  string `json:"product_uid"`
	Name        string `json:"name"`
	FullURL     string `json:"full_url"`
	ImageURL    string `json:"image"`
	IsAvailable bool   `json:"is_available"`
	RetailPrice struct {
		Price float64 `json:"price"`
	} `json:"retail_price"`
	UnitPrice struct {
		Measure string  `json:"measure"`
		Price   float64 `json:"price"`
	} `json:"unit_price"`
	Promotions []struct {
		PromotionDescription string `json:"promotion_description"`
	} `json:"promotions"`
}

type searchResponse struct {
	Products []apiProduct `json:"products"`
}

type category struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
}

// Datasource uses the Sainsbury's JSON API.
type Datasource struct {
	cookies    []*http.Cookie
	httpClient *http.Client
	apiBase    string
}

// NewDatasource creates a new Sainsbury's API datasource.
func NewDatasource() *Datasource {
	return &Datasource{
		httpClient: &http.Client{},
		apiBase:    apiBase,
	}
}

// NewDatasourceWithURL creates a Sainsbury's datasource pointing at a custom URL (for testing).
func NewDatasourceWithURL(baseURL string) *Datasource {
	return &Datasource{
		httpClient: &http.Client{},
		apiBase:    baseURL,
	}
}

// SetCookies sets session cookies to inject into every API request.
func (s *Datasource) SetCookies(cookies []*http.Cookie) { s.cookies = cookies }

// ID returns the supermarket identifier.
func (s *Datasource) ID() datasource.SupermarketID { return datasource.Sainsburys }

// Name returns the human-readable name.
func (s *Datasource) Name() string { return "Sainsbury's" }

// SearchProducts searches for products using the Sainsbury's API.
func (s *Datasource) SearchProducts(
	ctx context.Context,
	query string,
) ([]datasource.Product, error) {
	apiURL := s.apiBase + "/product/v1/product?" + url.Values{
		"filter[keyword]": {query},
		"page_number":     {"1"},
		"page_size":       {"24"},
		"sort_order":      {"FAVOURITES_FIRST"},
	}.Encode()

	body, err := s.apiRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("sainsburys search: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	var resp searchResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("sainsburys: decode search response: %w", err)
	}

	products := make([]datasource.Product, 0, len(resp.Products))
	for _, ap := range resp.Products {
		products = append(products, convertProduct(ap))
	}
	return products, nil
}

// GetProductDetails fetches details for a specific product.
func (s *Datasource) GetProductDetails(
	ctx context.Context,
	productID string,
) (*datasource.Product, error) {
	apiURL := s.apiBase + "/product/v1/product/" +
		url.PathEscape(productID)

	body, err := s.apiRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("sainsburys product: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	var ap apiProduct
	if err := json.NewDecoder(body).Decode(&ap); err != nil {
		return nil, fmt.Errorf("sainsburys: decode product: %w", err)
	}
	p := convertProduct(ap)
	return &p, nil
}

// BrowseCategories returns the top-level grocery categories.
func (s *Datasource) BrowseCategories(
	ctx context.Context,
) ([]datasource.Category, error) {
	apiURL := s.apiBase + "/product/categories/tree"

	body, err := s.apiRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("sainsburys categories: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	var cats []category
	if err := json.NewDecoder(body).Decode(&cats); err != nil {
		return nil, fmt.Errorf("sainsburys: decode categories: %w", err)
	}

	categories := make([]datasource.Category, 0, len(cats))
	for _, c := range cats {
		categories = append(categories, datasource.Category{
			ID:          c.CategoryID,
			Name:        c.Name,
			URL:         baseURL + c.URL,
			Supermarket: datasource.Sainsburys,
		})
	}
	return categories, nil
}

func (s *Datasource) apiRequest(
	ctx context.Context,
	apiURL string,
) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	scraper.SetBrowserHeaders(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", baseURL+"/shop/gb/groceries")
	for _, c := range s.cookies {
		req.AddCookie(c)
		if strings.HasPrefix(c.Name, "WC_AUTHENTICATION_") {
			req.Header.Set("wcauthtoken", c.Value)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, apiURL)
	}
	return resp.Body, nil
}

func convertProduct(ap apiProduct) datasource.Product {
	p := datasource.Product{
		ID:          ap.ProductUID,
		Supermarket: datasource.Sainsburys,
		Name:        ap.Name,
		Price:       ap.RetailPrice.Price,
		Currency:    "GBP",
		Available:   ap.IsAvailable,
		ImageURL:    ap.ImageURL,
	}
	if ap.FullURL != "" {
		p.URL = baseURL + ap.FullURL
	}
	if ap.UnitPrice.Price > 0 {
		p.PricePerUnit = fmt.Sprintf("%.1fp/%s",
			ap.UnitPrice.Price*100, ap.UnitPrice.Measure)
	}
	if len(ap.Promotions) > 0 {
		var promos []string
		for _, promo := range ap.Promotions {
			if promo.PromotionDescription != "" {
				promos = append(promos, promo.PromotionDescription)
			}
		}
		p.Promotion = strings.Join(promos, "; ")
	}
	return p
}

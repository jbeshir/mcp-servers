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

// stringOrArray handles JSON fields that may be a string or an array of strings.
type stringOrArray string

func (s *stringOrArray) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = stringOrArray(str)
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = stringOrArray(strings.Join(arr, "\n"))
		return nil
	}
	*s = ""
	return nil
}

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
	Description stringOrArray `json:"description"`
	Ingredients stringOrArray `json:"ingredients"`
	Nutrition   *struct {
		Per100g     map[string]string `json:"per_100g"`
		PerPortion  map[string]string `json:"per_portion"`
		PortionSize string            `json:"portion_size"`
	} `json:"nutrition"`
}

type searchResponse struct {
	Products []apiProduct `json:"products"`
}

// categoryTreeResponse is the top-level response from the categories/tree endpoint.
type categoryTreeResponse struct {
	Hierarchy categoryNode `json:"category_hierarchy"`
}

// categoryNode represents a node in the category hierarchy.
type categoryNode struct {
	Slug     string         `json:"s"`
	Name     string         `json:"n"`
	Children []categoryNode `json:"c"`
}

// Config holds optional overrides for a Sainsbury's datasource.
// Zero values use the built-in defaults.
type Config struct {
	BaseURL string
}

// Datasource uses the Sainsbury's JSON API.
type Datasource struct {
	cookies    []*http.Cookie
	httpClient *http.Client
	apiBase    string
}

// NewDatasource creates a new Sainsbury's API datasource.
func NewDatasource(cfg Config, httpClient *http.Client) *Datasource {
	base := apiBase
	if cfg.BaseURL != "" {
		base = cfg.BaseURL
	}
	return &Datasource{
		httpClient: httpClient,
		apiBase:    base,
	}
}

func (s *Datasource) SetCookies(cookies []*http.Cookie) { s.cookies = cookies }

func (s *Datasource) ID() datasource.SupermarketID { return datasource.Sainsburys }
func (s *Datasource) Name() string                 { return "Sainsbury's" }
func (s *Datasource) Description() string          { return "One of the UK's largest supermarket chains" }

// CheckSession validates whether cached cookies represent a valid session.
// It makes a minimal search request because the categories endpoint works
// without auth, so it can't detect stale cookies that cause 401s on search.
func (s *Datasource) CheckSession(ctx context.Context) bool {
	if len(s.cookies) == 0 {
		return true
	}
	apiURL := s.apiBase + "/product/v1/product?" + url.Values{
		"filter[keyword]": {"milk"},
		"page_number":     {"1"},
		"page_size":       {"1"},
	}.Encode()
	body, err := s.apiRequest(ctx, apiURL)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	var result json.RawMessage
	return json.NewDecoder(body).Decode(&result) == nil
}

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

	var resp categoryTreeResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("sainsburys: decode categories: %w", err)
	}

	categories := make([]datasource.Category, 0, len(resp.Hierarchy.Children))
	for _, c := range resp.Hierarchy.Children {
		slug := c.Slug
		categories = append(categories, datasource.Category{
			ID:          scraper.LastPathSegment(slug),
			Name:        c.Name,
			URL:         baseURL + "/shop/" + slug,
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
		Description: string(ap.Description),
		Ingredients: string(ap.Ingredients),
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
	if ap.Nutrition != nil && len(ap.Nutrition.Per100g) > 0 {
		p.Nutrition = &datasource.NutritionInfo{
			Per100g:     ap.Nutrition.Per100g,
			PerPortion:  ap.Nutrition.PerPortion,
			PortionSize: ap.Nutrition.PortionSize,
		}
	}
	return p
}

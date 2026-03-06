// Package shopify provides datasources for supermarkets using the Shopify platform.
package shopify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

// Config holds the per-store settings for a Shopify datasource.
type Config struct {
	ID          datasource.SupermarketID
	Name        string
	Description string
	BaseURL     string // e.g. "https://hiyou.co"
}

// Datasource implements datasource.Datasource for Shopify stores.
type Datasource struct {
	cfg        Config
	httpClient *http.Client
}

// NewDatasource creates a Datasource with a default HTTP client.
func NewDatasource(cfg Config) *Datasource {
	return &Datasource{cfg: cfg, httpClient: http.DefaultClient}
}

// NewDatasourceWithClient creates a Datasource with a custom HTTP client (for testing).
func NewDatasourceWithClient(cfg Config, client *http.Client) *Datasource {
	return &Datasource{cfg: cfg, httpClient: client}
}

// ID returns the supermarket identifier.
func (d *Datasource) ID() datasource.SupermarketID { return d.cfg.ID }

// Name returns the human-readable name.
func (d *Datasource) Name() string { return d.cfg.Name }

// Description returns a short description of the supermarket.
func (d *Datasource) Description() string { return d.cfg.Description }

// searchResponse is the top-level Shopify predictive search response.
type searchResponse struct {
	Resources struct {
		Results struct {
			Products []searchProduct `json:"products"`
		} `json:"results"`
	} `json:"resources"`
}

// searchProduct is a product returned by Shopify's predictive search.
type searchProduct struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Handle        string `json:"handle"`
	Price         string `json:"price"`
	Available     bool   `json:"available"`
	URL           string `json:"url"`
	Image         string `json:"image"`
	Type          string `json:"type"`
	FeaturedImage struct {
		URL string `json:"url"`
	} `json:"featured_image"`
}

// SearchProducts searches the store using Shopify's predictive search API.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	u := d.cfg.BaseURL + "/search/suggest.json?" + url.Values{
		"q":                {query},
		"resources[type]":  {"product"},
		"resources[limit]": {"10"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request returned status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	products := make([]datasource.Product, 0, len(sr.Resources.Results.Products))
	for _, p := range sr.Resources.Results.Products {
		products = append(products, d.convertSearchProduct(p))
	}
	return products, nil
}

func (d *Datasource) convertSearchProduct(p searchProduct) datasource.Product {
	price, _ := strconv.ParseFloat(p.Price, 64)
	imageURL := p.Image
	if imageURL == "" {
		imageURL = p.FeaturedImage.URL
	}
	productURL := p.URL
	if productURL != "" && !strings.HasPrefix(productURL, "http") {
		productURL = d.cfg.BaseURL + productURL
	}
	return datasource.Product{
		ID:          p.Handle,
		Supermarket: d.cfg.ID,
		Name:        p.Title,
		Price:       price,
		Currency:    "GBP",
		ImageURL:    imageURL,
		URL:         productURL,
		Available:   p.Available,
	}
}

// productResponse is the top-level Shopify product detail response.
type productResponse struct {
	Product productDetail `json:"product"`
}

// productDetail is the full product from Shopify's product API.
type productDetail struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Handle      string    `json:"handle"`
	BodyHTML    string    `json:"body_html"`
	ProductType string    `json:"product_type"`
	Variants    []variant `json:"variants"`
	Images      []image   `json:"images"`
}

type variant struct {
	Price      string  `json:"price"`
	Weight     float64 `json:"weight"`
	WeightUnit string  `json:"weight_unit"`
}

type image struct {
	Src string `json:"src"`
}

// GetProductDetails fetches a single product by its handle.
func (d *Datasource) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	u := d.cfg.BaseURL + "/products/" + url.PathEscape(productID) + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating product request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("product request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product request returned status %d", resp.StatusCode)
	}

	var pr productResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decoding product response: %w", err)
	}

	p := pr.Product
	price := 0.0
	weight := ""
	if len(p.Variants) > 0 {
		price, _ = strconv.ParseFloat(p.Variants[0].Price, 64)
		if p.Variants[0].Weight > 0 {
			weight = fmt.Sprintf("%.0f%s", p.Variants[0].Weight, p.Variants[0].WeightUnit)
		}
	}
	imageURL := ""
	if len(p.Images) > 0 {
		imageURL = p.Images[0].Src
	}

	result := &datasource.Product{
		ID:          p.Handle,
		Supermarket: d.cfg.ID,
		Name:        p.Title,
		Price:       price,
		Currency:    "GBP",
		ImageURL:    imageURL,
		URL:         d.cfg.BaseURL + "/products/" + p.Handle,
		Available:   true,
		Weight:      weight,
	}
	if p.BodyHTML != "" {
		result.Description = stripHTML(p.BodyHTML)
	}

	return result, nil
}

// stripHTML parses an HTML fragment and returns its text content.
func stripHTML(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	return scraper.TextContent(doc)
}

// collectionsResponse is the top-level Shopify collections response.
type collectionsResponse struct {
	Collections []collection `json:"collections"`
}

type collection struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Handle string `json:"handle"`
}

// BrowseCategories returns the store's collections as categories.
func (d *Datasource) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	u := d.cfg.BaseURL + "/collections.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating collections request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collections request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("collections request returned status %d", resp.StatusCode)
	}

	var cr collectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decoding collections response: %w", err)
	}

	categories := make([]datasource.Category, 0, len(cr.Collections))
	for _, c := range cr.Collections {
		categories = append(categories, datasource.Category{
			ID:          strconv.FormatInt(c.ID, 10),
			Name:        c.Title,
			URL:         d.cfg.BaseURL + "/collections/" + c.Handle,
			Supermarket: d.cfg.ID,
		})
	}
	return categories, nil
}

// Package datasource defines the interface and types for supermarket product data sources.
package datasource

import (
	"context"
	"net/http"
)

// SupermarketID identifies a supermarket.
type SupermarketID string

const (
	// Tesco is the Tesco supermarket.
	Tesco SupermarketID = "tesco"
	// Sainsburys is the Sainsbury's supermarket.
	Sainsburys SupermarketID = "sainsburys"
	// Ocado is the Ocado supermarket.
	Ocado SupermarketID = "ocado"
	// Morrisons is the Morrisons supermarket.
	Morrisons SupermarketID = "morrisons"
	// Hiyou is the HiYoU Asian supermarket.
	Hiyou SupermarketID = "hiyou"
	// TukTukMart is the Tuk Tuk Mart Asian supermarket.
	TukTukMart SupermarketID = "tuktukmart"
	// Morueats is the Morueats Asian grocery store.
	Morueats SupermarketID = "morueats"
)

// AllSupermarkets is the list of all supported supermarket IDs.
var AllSupermarkets = []SupermarketID{Tesco, Sainsburys, Ocado, Morrisons, Hiyou, TukTukMart, Morueats}

// Product represents a supermarket product.
type Product struct {
	ID           string        `json:"id"`
	Supermarket  SupermarketID `json:"supermarket"`
	Name         string        `json:"name"`
	Price        float64       `json:"price"`
	PricePerUnit string        `json:"pricePerUnit"`
	Currency     string        `json:"currency"`
	ImageURL     string        `json:"imageURL,omitempty"`
	URL          string        `json:"url"`
	Available    bool          `json:"available"`
	Weight       string        `json:"weight,omitempty"`
	Promotion    string        `json:"promotion,omitempty"`
}

// Category represents a product category in a supermarket.
type Category struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	Supermarket SupermarketID `json:"supermarket"`
}

// SearchResult holds the results of a product search for a single supermarket.
type SearchResult struct {
	Supermarket SupermarketID `json:"supermarket"`
	Products    []Product     `json:"products"`
	TotalCount  int           `json:"totalCount"`
	Error       string        `json:"error,omitempty"`
}

// Datasource provides access to a supermarket's product data.
type Datasource interface {
	ID() SupermarketID
	Name() string
	Description() string
	SearchProducts(ctx context.Context, query string) ([]Product, error)
	GetProductDetails(ctx context.Context, productID string) (*Product, error)
	BrowseCategories(ctx context.Context) ([]Category, error)
}

// AuthDatasource is a Datasource that supports session cookie injection
// and session validation.
type AuthDatasource interface {
	Datasource
	SetCookies(cookies []*http.Cookie)
	CheckSession(ctx context.Context) bool
}

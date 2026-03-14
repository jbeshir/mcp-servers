// Package datasource defines the interface and types for supermarket product data sources.
package datasource

import (
	"context"
	"errors"
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
	// Asda is the Asda supermarket.
	Asda SupermarketID = "asda"
	// Waitrose is the Waitrose supermarket.
	Waitrose SupermarketID = "waitrose"
	// Hiyou is the HiYoU Asian supermarket.
	Hiyou SupermarketID = "hiyou"
	// TukTukMart is the Tuk Tuk Mart Asian supermarket.
	TukTukMart SupermarketID = "tuktukmart"
	// Morueats is the Morueats Asian grocery store.
	Morueats SupermarketID = "morueats"
)

// AllSupermarkets is the list of all supported supermarket IDs.
var AllSupermarkets = []SupermarketID{Tesco, Sainsburys, Ocado, Morrisons, Asda, Waitrose, Hiyou, TukTukMart, Morueats}

// NutritionInfo holds nutritional information for a product.
type NutritionInfo struct {
	Per100g     map[string]string `json:"per100g,omitempty"`
	PerPortion  map[string]string `json:"perPortion,omitempty"`
	PortionSize string            `json:"portionSize,omitempty"`
}

// Product represents a supermarket product.
type Product struct {
	ID           string         `json:"id"`
	Supermarket  SupermarketID  `json:"supermarket"`
	Name         string         `json:"name"`
	Price        float64        `json:"price"`
	PricePerUnit string         `json:"pricePerUnit"`
	Currency     string         `json:"currency"`
	ImageURL     string         `json:"imageURL,omitempty"`
	URL          string         `json:"url"`
	Available    bool           `json:"available"`
	Weight       string         `json:"weight,omitempty"`
	Promotion    string         `json:"promotion,omitempty"`
	Description  string         `json:"description,omitempty"`
	Ingredients  string         `json:"ingredients,omitempty"`
	Nutrition    *NutritionInfo `json:"nutrition,omitempty"`
	DietaryInfo  []string       `json:"dietaryInfo,omitempty"`
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

// ProductSource provides access to a supermarket's product catalog.
type ProductSource interface {
	ID() SupermarketID
	Name() string
	Description() string
	SearchProducts(ctx context.Context, query string) ([]Product, error)
	GetProductDetails(ctx context.Context, productID string) (*Product, error)
	BrowseCategories(ctx context.Context) ([]Category, error)
}

// OrderHistorySource provides access to a supermarket's order history.
type OrderHistorySource interface {
	GetOrderHistory(ctx context.Context, page int) (*OrderHistoryResult, error)
}

// BasketSource provides access to a supermarket's basket management.
type BasketSource interface {
	GetBasket(ctx context.Context) (*Basket, error)
	UpdateBasketItem(ctx context.Context, productID string, quantity int) (*Basket, error)
}

// AuthProductSource is a ProductSource that supports session cookie injection
// and session validation.
type AuthProductSource interface {
	ProductSource
	SetCookies(cookies []*http.Cookie)
	// CheckSession returns true if the session is usable.
	// When no cookies have been set, it returns true (no session to validate).
	CheckSession(ctx context.Context) bool
}

// ErrSessionExpired signals that a datasource's session has expired
// and re-authentication is needed.
var ErrSessionExpired = errors.New("session expired")

// OrderItem represents a single item in an order.
type OrderItem struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	ImageURL  string `json:"imageURL,omitempty"`
}

// Order represents a past grocery order.
type Order struct {
	ID             string        `json:"id"`
	Supermarket    SupermarketID `json:"supermarket"`
	Status         string        `json:"status"`
	Date           string        `json:"date"`
	DeliverySlot   string        `json:"deliverySlot,omitempty"`
	ShoppingMethod string        `json:"shoppingMethod,omitempty"`
	TotalPrice     float64       `json:"totalPrice"`
	TotalItems     int           `json:"totalItems"`
	Currency       string        `json:"currency"`
	Items          []OrderItem   `json:"items"`
}

// OrderHistoryResult holds order history results and pagination info.
type OrderHistoryResult struct {
	Supermarket SupermarketID `json:"supermarket"`
	Orders      []Order       `json:"orders"`
	Total       *int          `json:"total,omitempty"`
	Page        int           `json:"page"`
	PageSize    int           `json:"pageSize"`
	Error       string        `json:"error,omitempty"`
}

// BasketItem represents an item in the shopping basket.
type BasketItem struct {
	ProductID string  `json:"productId"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Cost      float64 `json:"cost"`
	Price     float64 `json:"price"`
	ImageURL  string  `json:"imageURL,omitempty"`
	Promotion string  `json:"promotion,omitempty"`
}

// Basket represents the current shopping basket.
type Basket struct {
	Supermarket SupermarketID `json:"supermarket"`
	Items       []BasketItem  `json:"items"`
	TotalPrice  float64       `json:"totalPrice"`
	TotalItems  int           `json:"totalItems"`
	Currency    string        `json:"currency"`
}

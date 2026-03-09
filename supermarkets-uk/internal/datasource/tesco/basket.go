package tesco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

const xapiURL = "https://xapi.tesco.com/"

// xapiKey is the public API key used for Tesco's GraphQL gateway.
var xapiKey = "TvOSZJHlEk0pjniDGQFAc9Q59WGAR4dA" //nolint:gosec // Public API key, not a secret.

const trolleyURL = baseURL + "/groceries/en-GB/trolley"

// trolleyWaitSelector waits for product list items to render.
const trolleyWaitSelector = `[data-testid="product-list-item"]`

const updateBasketMutation = `mutation UpdateBasket($items: [BasketLineItemInputType], $orderId: ID) {
  basket(items: $items, orderId: $orderId) {
    id
    orderId
    totalPrice
    guidePrice
    splitView {
      id
      guidePrice
      totalPrice
      totalItems
      items {
        id
        quantity
        cost
        unit
        product {
          id
          title
          defaultImageUrl
          price {
            actual
            unitPrice
            unitOfMeasure
          }
          promotions {
            description
          }
        }
      }
    }
  }
}`

type graphQLRequest struct {
	OperationName string      `json:"operationName"`
	Query         string      `json:"query"`
	Variables     interface{} `json:"variables"`
	Extensions    struct {
		MFEName string `json:"mfeName"`
	} `json:"extensions"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResult struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

type basketResponseData struct {
	Basket basketGraphQLData `json:"basket"`
}

type basketGraphQLData struct {
	ID         string        `json:"id"`
	OrderID    string        `json:"orderId"`
	TotalPrice float64       `json:"totalPrice"`
	GuidePrice float64       `json:"guidePrice"`
	SplitView  []basketSplit `json:"splitView"`
}

type basketSplit struct {
	ID         string       `json:"id"`
	GuidePrice float64      `json:"guidePrice"`
	TotalPrice float64      `json:"totalPrice"`
	TotalItems int          `json:"totalItems"`
	Items      []basketItem `json:"items"`
}

type basketItem struct {
	ID       string  `json:"id"`
	Quantity int     `json:"quantity"`
	Cost     float64 `json:"cost"`
	Unit     string  `json:"unit"`
	Product  struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		DefaultImageURL string `json:"defaultImageUrl"`
		Price           struct {
			Actual        float64 `json:"actual"`
			UnitPrice     float64 `json:"unitPrice"`
			UnitOfMeasure string  `json:"unitOfMeasure"`
		} `json:"price"`
		Promotions []struct {
			Description string `json:"description"`
		} `json:"promotions"`
	} `json:"product"`
}

// cachedBasket is the Apollo cache entry for the basket on the trolley page.
type cachedBasket struct {
	ID         string  `json:"id"`
	OrderID    string  `json:"orderId"`
	TotalPrice float64 `json:"totalPrice"`
	GuidePrice float64 `json:"guidePrice"`
	SplitView  []struct {
		Ref string `json:"__ref"`
	} `json:"splitView"`
	Items []struct {
		Ref string `json:"__ref"`
	} `json:"items"`
}

type cachedBasketSummary struct {
	ID         string  `json:"id"`
	TotalPrice float64 `json:"totalPrice"`
	GuidePrice float64 `json:"guidePrice"`
	TotalItems int     `json:"totalItems"`
	Items      []struct {
		Ref string `json:"__ref"`
	} `json:"items"`
}

type cachedBasketItem struct {
	ID       string  `json:"id"`
	Quantity int     `json:"quantity"`
	Cost     float64 `json:"cost"`
	Unit     string  `json:"unit"`
	Product  struct {
		Ref string `json:"__ref"`
	} `json:"product"`
}

type cachedBasketProduct struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DefaultImageURL string `json:"defaultImageUrl"`
	Price           struct {
		Actual        float64 `json:"actual"`
		UnitPrice     float64 `json:"unitPrice"`
		UnitOfMeasure string  `json:"unitOfMeasure"`
	} `json:"price"`
	Promotions []struct {
		Ref string `json:"__ref"`
	} `json:"promotions"`
}

type cachedPromotion struct {
	Description string `json:"description"`
}

// GetBasket retrieves the current shopping basket by parsing the trolley page.
func (d *Datasource) GetBasket(ctx context.Context) (*datasource.Basket, error) {
	body, err := d.browser.Fetch(ctx, trolleyURL, d.cookies, trolleyWaitSelector)
	if err != nil {
		return nil, fmt.Errorf("tesco basket fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck

	return ParseBasket(body)
}

// ParseBasket parses a Tesco trolley page and extracts the basket from
// the embedded Apollo cache.
func ParseBasket(r io.Reader) (*datasource.Basket, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("tesco: read trolley page: %w", err)
	}

	cache, err := extractApolloCache(data)
	if err != nil {
		return nil, err
	}

	return resolveBasket(cache)
}

func findCachedBasket(
	cache map[string]json.RawMessage,
) (*cachedBasket, error) {
	var basketRef struct {
		Ref string `json:"__ref"`
	}
	if raw, ok := cache["ROOT_QUERY"]; ok {
		var rootQuery map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rootQuery); err == nil {
			if bRaw, ok := rootQuery["basket"]; ok {
				_ = json.Unmarshal(bRaw, &basketRef)
			}
		}
	}
	if basketRef.Ref == "" {
		return nil, fmt.Errorf("tesco: no basket found in Apollo cache")
	}

	raw, ok := cache[basketRef.Ref]
	if !ok {
		return nil, fmt.Errorf("tesco: basket ref %q not found", basketRef.Ref)
	}
	var cb cachedBasket
	if err := json.Unmarshal(raw, &cb); err != nil {
		return nil, fmt.Errorf("tesco: parse cached basket: %w", err)
	}
	return &cb, nil
}

func resolveBasket(
	cache map[string]json.RawMessage,
) (*datasource.Basket, error) {
	cb, err := findCachedBasket(cache)
	if err != nil {
		return nil, err
	}

	basket := &datasource.Basket{
		Supermarket: datasource.Tesco,
		TotalPrice:  cb.TotalPrice,
		Currency:    "GBP",
	}

	// Resolve items from the first splitView (GHS summary).
	if len(cb.SplitView) > 0 {
		svRef := cb.SplitView[0].Ref
		if svRaw, ok := cache[svRef]; ok {
			var sv cachedBasketSummary
			if err := json.Unmarshal(svRaw, &sv); err == nil {
				basket.TotalPrice = sv.TotalPrice
				basket.TotalItems = sv.TotalItems
				for _, itemRef := range sv.Items {
					bi := resolveBasketItem(cache, itemRef.Ref)
					if bi != nil {
						basket.Items = append(basket.Items, *bi)
					}
				}
			}
		}
	}

	return basket, nil
}

func resolveBasketItem(
	cache map[string]json.RawMessage, ref string,
) *datasource.BasketItem {
	raw, ok := cache[ref]
	if !ok {
		return nil
	}
	var ci cachedBasketItem
	if err := json.Unmarshal(raw, &ci); err != nil {
		return nil
	}

	item := &datasource.BasketItem{
		Quantity: ci.Quantity,
		Cost:     ci.Cost,
	}

	if ci.Product.Ref != "" {
		if pRaw, ok := cache[ci.Product.Ref]; ok {
			var cp cachedBasketProduct
			if err := json.Unmarshal(pRaw, &cp); err == nil {
				item.ProductID = cp.ID
				item.Name = cp.Title
				item.Price = cp.Price.Actual
				item.ImageURL = cp.DefaultImageURL
				if len(cp.Promotions) > 0 {
					if prRaw, ok := cache[cp.Promotions[0].Ref]; ok {
						var pr cachedPromotion
						if json.Unmarshal(prRaw, &pr) == nil {
							item.Promotion = pr.Description
						}
					}
				}
			}
		}
	}

	return item
}

// UpdateBasketItem adds, updates, or removes (quantity=0) a product in the basket.
// It uses the browser to load the trolley page (which refreshes the OAuth token),
// reads the fresh access token from browser cookies, and calls the GraphQL API.
func (d *Datasource) UpdateBasketItem(
	ctx context.Context, productID string, quantity int,
) (*datasource.Basket, error) {
	body, token, err := d.browser.FetchAndReadCookie(
		ctx, trolleyURL, d.cookies, "OAuth.AccessToken", trolleyWaitSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("tesco basket fetch for update: %w", err)
	}
	defer body.Close() //nolint:errcheck

	if token == "" {
		return nil, fmt.Errorf(
			"tesco: no access token in browser cookies — session may have expired",
		)
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("tesco: read trolley page: %w", err)
	}

	cache, err := extractApolloCache(data)
	if err != nil {
		return nil, err
	}

	orderID, err := extractOrderID(cache)
	if err != nil {
		return nil, err
	}

	type updateItem struct {
		ID                 string `json:"id"`
		NewValue           int    `json:"newValue"`
		NewUnitChoice      string `json:"newUnitChoice"`
		Adjustment         bool   `json:"adjustment"`
		SubstitutionOption string `json:"substitutionOption,omitempty"`
	}
	type updateVars struct {
		Items   []updateItem `json:"items"`
		OrderID string       `json:"orderId"`
	}

	item := updateItem{
		ID:            productID,
		NewValue:      quantity,
		NewUnitChoice: "pcs",
		Adjustment:    false,
	}
	if quantity > 0 {
		item.SubstitutionOption = "FindSuitableAlternative"
	}

	req := graphQLRequest{
		OperationName: "UpdateBasket",
		Query:         updateBasketMutation,
		Variables: updateVars{
			Items:   []updateItem{item},
			OrderID: orderID,
		},
	}
	req.Extensions.MFEName = "mfe-basket-manager"

	resp, err := d.graphQL(ctx, token, req)
	if err != nil {
		return nil, fmt.Errorf("tesco: update basket: %w", err)
	}

	return parseGraphQLBasket(resp)
}

func extractOrderID(
	cache map[string]json.RawMessage,
) (string, error) {
	cb, err := findCachedBasket(cache)
	if err != nil {
		return "", err
	}
	if cb.OrderID != "" {
		return cb.OrderID, nil
	}
	return cb.ID, nil
}

func (d *Datasource) graphQL(
	ctx context.Context, token string, gqlReq graphQLRequest,
) (json.RawMessage, error) {
	// Tesco xapi expects a JSON array of operations.
	body, err := json.Marshal([]graphQLRequest{gqlReq})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, xapiURL, bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-apikey", xapiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"xapi returned %d: %s", resp.StatusCode, respBody,
		)
	}

	var results []graphQLResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("parse xapi response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty xapi response")
	}

	result := results[0]
	if len(result.Errors) > 0 {
		msgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf(
			"graphql errors: %s", strings.Join(msgs, "; "),
		)
	}

	return result.Data, nil
}

func parseGraphQLBasket(
	data json.RawMessage,
) (*datasource.Basket, error) {
	var resp basketResponseData
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("tesco: parse basket response: %w", err)
	}

	basket := &datasource.Basket{
		Supermarket: datasource.Tesco,
		TotalPrice:  resp.Basket.TotalPrice,
		Currency:    "GBP",
	}

	if len(resp.Basket.SplitView) > 0 {
		sv := resp.Basket.SplitView[0]
		basket.TotalPrice = sv.TotalPrice
		basket.TotalItems = sv.TotalItems
		for _, item := range sv.Items {
			bi := datasource.BasketItem{
				ProductID: item.Product.ID,
				Name:      item.Product.Title,
				Quantity:  item.Quantity,
				Cost:      item.Cost,
				Price:     item.Product.Price.Actual,
				ImageURL:  item.Product.DefaultImageURL,
			}
			if len(item.Product.Promotions) > 0 {
				bi.Promotion = item.Product.Promotions[0].Description
			}
			basket.Items = append(basket.Items, bi)
		}
	}

	return basket, nil
}

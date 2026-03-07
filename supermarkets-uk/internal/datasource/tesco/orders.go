package tesco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

const ordersURL = baseURL + "/groceries/en-GB/orders/recent"

// ordersWaitSelector waits for the order tabs to render before capturing HTML.
const ordersWaitSelector = `[data-testid="myorder-tabs"]`

// apolloCache is the top-level JSON structure embedded in the page.
type apolloCache struct {
	Orchestrator struct {
		Props struct {
			Cache map[string]json.RawMessage `json:"apolloCache"`
		} `json:"props"`
	} `json:"mfe-orchestrator"`
}

type orderSearchResult struct {
	Orders []struct {
		Ref string `json:"__ref"`
	} `json:"orders"`
	Info struct {
		Total    int `json:"total"`
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
	} `json:"info"`
}

type cachedOrder struct {
	OrderNo        string  `json:"orderNo"`
	Status         string  `json:"status"`
	TotalPrice     float64 `json:"totalPrice"`
	TotalItems     int     `json:"totalItems"`
	ShoppingMethod string  `json:"shoppingMethod"`
	Localisation   struct {
		Currency struct {
			ISO string `json:"iso"`
		} `json:"currency"`
	} `json:"localisation"`
	Slot  refOrValue `json:"slot"`
	Items []struct {
		Ref string `json:"__ref"`
	} `json:"items"`
}

type refOrValue struct {
	Ref  string          `json:"__ref"`
	Data json.RawMessage `json:"-"`
}

func (r *refOrValue) UnmarshalJSON(data []byte) error {
	// Try as ref first.
	type plain struct {
		Ref string `json:"__ref"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err == nil && p.Ref != "" {
		r.Ref = p.Ref
		return nil
	}
	r.Data = data
	return nil
}

type cachedSlot struct {
	Start  string  `json:"start"`
	End    string  `json:"end"`
	Charge float64 `json:"charge"`
}

type cachedOrderItem struct {
	Quantity int `json:"quantity"`
	Product  struct {
		Ref string `json:"__ref"`
	} `json:"product"`
}

type cachedProduct struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DefaultImageURL string `json:"defaultImageUrl"`
}

// orderSearchKeyRe matches the Apollo cache key for the past orders query.
var orderSearchKeyRe = regexp.MustCompile(
	`^orderSearch\(\{.*"statuses":\["Previous"\].*\}\)$`,
)

// GetOrderHistory retrieves past order history from the Tesco orders page.
func (d *Datasource) GetOrderHistory(
	ctx context.Context, page int,
) (*datasource.OrderHistoryResult, error) {
	url := ordersURL
	if page > 1 {
		url = ordersURL + "?page=" + strconv.Itoa(page)
	}

	body, err := d.browser.Fetch(ctx, url, d.cookies, ordersWaitSelector)
	if err != nil {
		return nil, fmt.Errorf("tesco orders fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseOrderHistory(body)
}

// ParseOrderHistory parses a Tesco orders page and extracts order
// history from the embedded Apollo cache.
func ParseOrderHistory(
	r io.Reader,
) (*datasource.OrderHistoryResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("tesco: read order page: %w", err)
	}

	cache, err := extractApolloCache(data)
	if err != nil {
		return nil, err
	}

	return resolveOrders(cache)
}

// scriptTagRe matches <script> tags and captures their content.
var scriptTagRe = regexp.MustCompile(`<script[^>]*>([\s\S]*?)</script>`)

// titleRe extracts the page title for diagnostic messages.
var titleRe = regexp.MustCompile(`<title>(.*?)</title>`)

func extractApolloCache(
	html []byte,
) (map[string]json.RawMessage, error) {
	matches := scriptTagRe.FindAllSubmatch(html, -1)
	for _, m := range matches {
		body := m[1]
		if len(body) < 100 {
			continue
		}

		var ac apolloCache
		if err := json.Unmarshal(body, &ac); err != nil {
			continue
		}
		if len(ac.Orchestrator.Props.Cache) > 0 {
			return ac.Orchestrator.Props.Cache, nil
		}
	}

	title := ""
	if m := titleRe.FindSubmatch(html); m != nil {
		title = string(m[1])
	}
	if bytes.Contains(html, []byte("login")) ||
		bytes.Contains(html, []byte("sign-in")) ||
		bytes.Contains(html, []byte("Sign in")) {
		return nil, fmt.Errorf(
			"tesco: page appears to be a login page (title: %q) "+
				"— session may have expired", title,
		)
	}
	return nil, fmt.Errorf(
		"tesco: no Apollo cache found in page (title: %q)", title,
	)
}

func resolveOrders(
	cache map[string]json.RawMessage,
) (*datasource.OrderHistoryResult, error) {
	search, err := findOrderSearch(cache)
	if err != nil {
		return nil, err
	}

	result := &datasource.OrderHistoryResult{
		Supermarket: datasource.Tesco,
		Total:       &search.Info.Total,
		Page:        search.Info.Page,
		PageSize:    search.Info.PageSize,
	}

	for _, ref := range search.Orders {
		order, err := resolveOrder(cache, ref.Ref)
		if err != nil {
			continue
		}
		result.Orders = append(result.Orders, *order)
	}

	return result, nil
}

// findOrderSearch locates the past-orders search result in the Apollo
// cache. It checks both top-level keys and keys nested under ROOT_QUERY.
func findOrderSearch(
	cache map[string]json.RawMessage,
) (*orderSearchResult, error) {
	if r := searchInMap(cache); r != nil {
		return r, nil
	}

	// The orderSearch key lives inside ROOT_QUERY.
	if raw, ok := cache["ROOT_QUERY"]; ok {
		var rootQuery map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rootQuery); err == nil {
			if r := searchInMap(rootQuery); r != nil {
				return r, nil
			}
		}
	}

	return nil, fmt.Errorf(
		"tesco: no past order search results in Apollo cache",
	)
}

func searchInMap(
	m map[string]json.RawMessage,
) *orderSearchResult {
	for key, raw := range m {
		if !orderSearchKeyRe.MatchString(key) {
			continue
		}
		var search orderSearchResult
		if err := json.Unmarshal(raw, &search); err != nil {
			continue
		}
		return &search
	}
	return nil
}

func resolveOrder(
	cache map[string]json.RawMessage, ref string,
) (*datasource.Order, error) {
	raw, ok := cache[ref]
	if !ok {
		return nil, fmt.Errorf("order ref %q not found", ref)
	}
	var co cachedOrder
	if err := json.Unmarshal(raw, &co); err != nil {
		return nil, fmt.Errorf("unmarshal order: %w", err)
	}

	order := &datasource.Order{
		ID:             co.OrderNo,
		Supermarket:    datasource.Tesco,
		Status:         co.Status,
		TotalPrice:     co.TotalPrice,
		TotalItems:     co.TotalItems,
		ShoppingMethod: co.ShoppingMethod,
		Currency:       co.Localisation.Currency.ISO,
	}
	if order.Currency == "" {
		order.Currency = "GBP"
	}

	// Resolve delivery slot.
	slot := resolveSlot(cache, co.Slot)
	if slot != nil {
		order.DeliverySlot = formatSlot(slot)
		order.Date = formatSlotDate(slot)
	}

	// Resolve items.
	for _, itemRef := range co.Items {
		item := resolveItem(cache, itemRef.Ref)
		if item != nil {
			order.Items = append(order.Items, *item)
		}
	}

	return order, nil
}

func resolveSlot(
	cache map[string]json.RawMessage, ref refOrValue,
) *cachedSlot {
	var raw json.RawMessage
	switch {
	case ref.Ref != "":
		var ok bool
		raw, ok = cache[ref.Ref]
		if !ok {
			return nil
		}
	case len(ref.Data) > 0:
		raw = ref.Data
	default:
		return nil
	}
	var s cachedSlot
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}

func formatSlot(s *cachedSlot) string {
	start, err1 := time.Parse(time.RFC3339, s.Start)
	end, err2 := time.Parse(time.RFC3339, s.End)
	if err1 != nil || err2 != nil {
		return ""
	}
	return fmt.Sprintf(
		"%s, %s–%s",
		start.Format("Mon 2 Jan"),
		start.Format("3:04pm"),
		end.Format("3:04pm"),
	)
}

func formatSlotDate(s *cachedSlot) string {
	t, err := time.Parse(time.RFC3339, s.Start)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func resolveItem(
	cache map[string]json.RawMessage, ref string,
) *datasource.OrderItem {
	raw, ok := cache[ref]
	if !ok {
		return nil
	}
	var ci cachedOrderItem
	if err := json.Unmarshal(raw, &ci); err != nil {
		return nil
	}

	item := &datasource.OrderItem{
		Quantity: ci.Quantity,
	}

	// Resolve product details.
	if ci.Product.Ref != "" {
		if pRaw, ok := cache[ci.Product.Ref]; ok {
			var cp cachedProduct
			if err := json.Unmarshal(pRaw, &cp); err == nil {
				item.ProductID = cp.ID
				item.Name = cp.Title
				item.ImageURL = cp.DefaultImageURL
			}
		}
	}

	return item
}

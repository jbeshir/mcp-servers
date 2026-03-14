// Package client orchestrates supermarket datasources with rate limiting and concurrent search.
package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/auth"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/asda"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/sainsburys"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/shopify"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/tesco"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/waitrose"
)

// SupermarketInfo describes a supported supermarket for the list tool.
type SupermarketInfo struct {
	ID          datasource.SupermarketID `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
}

// authResolver handles lazy authentication and re-authentication on session expiry.
type authResolver struct {
	mu      sync.Mutex
	inner   datasource.AuthProductSource
	authed  bool
	doLogin func(ctx context.Context) ([]*http.Cookie, error)
}

func (a *authResolver) ensureAuth(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.authed {
		return
	}

	id := a.inner.ID()
	log.Printf(
		"first use of %s — starting interactive login "+
			"(complete login in the browser window)...", id,
	)

	cookies, err := a.doLogin(ctx)
	if err != nil {
		log.Printf(
			"warning: login failed for %s: %v "+
				"(using unauthenticated mode)", id, err,
		)
		a.authed = true
		return
	}

	a.inner.SetCookies(cookies)
	log.Printf("login successful for %s", id)
	a.authed = true
}

func (a *authResolver) tryReauth(ctx context.Context, err error) bool {
	if !errors.Is(err, datasource.ErrSessionExpired) {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	id := a.inner.ID()
	log.Printf("session expired for %s — re-authenticating...", id)

	a.inner.SetCookies(nil)
	a.authed = false

	cookies, loginErr := a.doLogin(ctx)
	if loginErr != nil {
		log.Printf(
			"warning: re-login failed for %s: %v", id, loginErr,
		)
		a.authed = true
		return false
	}

	a.inner.SetCookies(cookies)
	log.Printf("re-login successful for %s", id)
	a.authed = true
	return true
}

// Client orchestrates multiple supermarket product sources.
type Client struct {
	products     map[datasource.SupermarketID]datasource.ProductSource
	orderHistory map[datasource.SupermarketID]datasource.OrderHistorySource
	baskets      map[datasource.SupermarketID]datasource.BasketSource
	auth         map[datasource.SupermarketID]*authResolver
	browser      *scraper.Browser
}

// Config holds configuration for creating a Client.
type Config struct {
	// Cookies holds pre-loaded session cookies per supermarket.
	Cookies map[datasource.SupermarketID][]*http.Cookie
	// LoginFlags is the set of supermarkets enabled for login.
	// NewClient derives which ones actually need interactive login
	// based on whether they have valid cached cookies.
	LoginFlags map[datasource.SupermarketID]bool
	// Store persists cookies obtained via interactive login.
	Store *auth.CookieStore
}

// NewClient creates a new client with all supermarket datasources.
// Supermarkets with cached cookies use them immediately. Supermarkets
// flagged for login but without cached cookies get lazy auth that
// triggers interactive login on first use.
func NewClient(cfg Config) *Client {
	browser := scraper.NewBrowser()

	httpClient := func() *http.Client {
		return scraper.NewRateLimitedClient(rate.NewLimiter(1, 1))
	}

	sources := []datasource.AuthProductSource{
		tesco.NewDatasource(browser, httpClient()),
		asda.NewDatasource(browser, httpClient()),
		waitrose.NewDatasource(browser),
		sainsburys.NewDatasource(sainsburys.Config{}, httpClient()),
		osp.NewOcado(osp.Config{}, httpClient()),
		osp.NewMorrisons(osp.Config{}, httpClient()),
	}

	c := &Client{
		products:     make(map[datasource.SupermarketID]datasource.ProductSource, len(sources)+3),
		orderHistory: make(map[datasource.SupermarketID]datasource.OrderHistorySource),
		baskets:      make(map[datasource.SupermarketID]datasource.BasketSource),
		auth:         make(map[datasource.SupermarketID]*authResolver),
		browser:      browser,
	}

	for _, ds := range sources {
		c.registerAuthDatasource(ds, cfg)
	}

	// Add plain (non-auth) datasources.
	plainSources := []datasource.ProductSource{
		shopify.NewHiyou(httpClient()),
		shopify.NewTukTukMart(httpClient()),
		shopify.NewMorueats(httpClient()),
	}
	for _, ds := range plainSources {
		c.products[ds.ID()] = ds
	}

	return c
}

func (c *Client) registerAuthDatasource(ds datasource.AuthProductSource, cfg Config) {
	id := ds.ID()

	needLogin := false
	if cfg.LoginFlags[id] {
		if cookies := cfg.Cookies[id]; len(cookies) > 0 {
			ds.SetCookies(cookies)
			if !ds.CheckSession(context.Background()) {
				log.Printf("cached session for %s is invalid, clearing", id)
				ds.SetCookies(nil)
				if cfg.Store != nil {
					_ = cfg.Store.Clear(id)
				}
				needLogin = true
			}
		} else {
			needLogin = true
		}
	}

	if needLogin && cfg.Store != nil {
		loginCfg, ok := auth.SupermarketLoginConfigs[id]
		if ok {
			store := cfg.Store
			smID := id
			doLogin := func(
				ctx context.Context,
			) ([]*http.Cookie, error) {
				cookies, err := auth.InteractiveLogin(
					ctx, smID, loginCfg,
				)
				if err != nil {
					return nil, err
				}
				if err := store.Save(smID, cookies); err != nil {
					log.Printf(
						"warning: failed to save "+
							"cookies for %s: %v",
						smID, err,
					)
				} else {
					log.Printf(
						"saved %d cookies for %s",
						len(cookies), smID,
					)
				}
				return cookies, nil
			}
			c.auth[id] = &authResolver{
				inner:   ds,
				doLogin: doLogin,
			}
		}
	}

	c.products[id] = ds

	if oh, ok := ds.(datasource.OrderHistorySource); ok {
		c.orderHistory[id] = oh
	}
	if b, ok := ds.(datasource.BasketSource); ok {
		c.baskets[id] = b
	}
}

// Close releases resources held by the client (e.g. headless browser).
func (c *Client) Close() {
	if c.browser != nil {
		c.browser.Close()
	}
}

func (c *Client) ensureAuth(ctx context.Context, id datasource.SupermarketID) {
	if ar, ok := c.auth[id]; ok {
		ar.ensureAuth(ctx)
	}
}

func withAuth[T any](
	c *Client, ctx context.Context, id datasource.SupermarketID,
	fn func() (T, error),
) (T, error) {
	c.ensureAuth(ctx, id)
	result, err := fn()
	if err != nil {
		if ar, ok := c.auth[id]; ok && ar.tryReauth(ctx, err) {
			return fn()
		}
	}
	return result, err
}

// SearchAll searches multiple supermarkets concurrently.
// If supermarkets is empty, all are searched.
// Individual failures are captured in SearchResult.Error.
func (c *Client) SearchAll(
	ctx context.Context,
	query string,
	supermarkets []datasource.SupermarketID,
) []datasource.SearchResult {
	targets := supermarkets
	if len(targets) == 0 {
		targets = datasource.AllSupermarkets
	}

	results := make([]datasource.SearchResult, len(targets))
	var wg sync.WaitGroup
	for i, id := range targets {
		ds, ok := c.products[id]
		if !ok {
			results[i] = datasource.SearchResult{
				Supermarket: id,
				Error:       fmt.Sprintf("unknown supermarket: %s", id),
			}
			continue
		}

		wg.Add(1)
		go func(idx int, ds datasource.ProductSource, sid datasource.SupermarketID) {
			defer wg.Done()

			products, err := withAuth(c, ctx, sid, func() ([]datasource.Product, error) {
				return ds.SearchProducts(ctx, query)
			})
			if err != nil {
				results[idx] = datasource.SearchResult{
					Supermarket: sid,
					Error:       err.Error(),
				}
				return
			}

			results[idx] = datasource.SearchResult{
				Supermarket: sid,
				Products:    products,
				TotalCount:  len(products),
			}
		}(i, ds, id)
	}

	wg.Wait()
	return results
}

// ListSupermarkets returns info about all supported supermarkets.
func (c *Client) ListSupermarkets() []SupermarketInfo {
	var infos []SupermarketInfo
	for _, id := range datasource.AllSupermarkets {
		if ds, ok := c.products[id]; ok {
			infos = append(infos, SupermarketInfo{
				ID:          id,
				Name:        ds.Name(),
				Description: ds.Description(),
			})
		}
	}
	return infos
}

// BrowseCategories retrieves categories for a supermarket.
func (c *Client) BrowseCategories(
	ctx context.Context,
	id datasource.SupermarketID,
) ([]datasource.Category, error) {
	ds, ok := c.products[id]
	if !ok {
		return nil, fmt.Errorf("unknown supermarket: %s", id)
	}
	return withAuth(c, ctx, id, func() ([]datasource.Category, error) {
		return ds.BrowseCategories(ctx)
	})
}

// GetProductDetails retrieves details for a specific product.
func (c *Client) GetProductDetails(
	ctx context.Context,
	id datasource.SupermarketID,
	productID string,
) (*datasource.Product, error) {
	ds, ok := c.products[id]
	if !ok {
		return nil, fmt.Errorf("unknown supermarket: %s", id)
	}
	return withAuth(c, ctx, id, func() (*datasource.Product, error) {
		return ds.GetProductDetails(ctx, productID)
	})
}

// GetOrderHistory retrieves order history for a supermarket.
func (c *Client) GetOrderHistory(
	ctx context.Context,
	id datasource.SupermarketID,
	page int,
) (*datasource.OrderHistoryResult, error) {
	ds, ok := c.orderHistory[id]
	if !ok {
		return nil, fmt.Errorf("order history is not supported for %s", id)
	}
	return withAuth(c, ctx, id, func() (*datasource.OrderHistoryResult, error) {
		return ds.GetOrderHistory(ctx, page)
	})
}

// GetBasket retrieves the current basket for a supermarket.
func (c *Client) GetBasket(
	ctx context.Context,
	id datasource.SupermarketID,
) (*datasource.Basket, error) {
	ds, ok := c.baskets[id]
	if !ok {
		return nil, fmt.Errorf("basket management is not supported for %s", id)
	}
	return withAuth(c, ctx, id, func() (*datasource.Basket, error) {
		return ds.GetBasket(ctx)
	})
}

// UpdateBasketItem adds, updates, or removes a product in the basket.
// Set quantity to 0 to remove.
func (c *Client) UpdateBasketItem(
	ctx context.Context,
	id datasource.SupermarketID,
	productID string,
	quantity int,
) (*datasource.Basket, error) {
	ds, ok := c.baskets[id]
	if !ok {
		return nil, fmt.Errorf("basket management is not supported for %s", id)
	}
	return withAuth(c, ctx, id, func() (*datasource.Basket, error) {
		return ds.UpdateBasketItem(ctx, productID, quantity)
	})
}

// ParseSupermarketIDs parses a comma-separated list of supermarket IDs.
func ParseSupermarketIDs(s string) []datasource.SupermarketID {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var ids []datasource.SupermarketID
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, datasource.SupermarketID(p))
		}
	}
	return ids
}

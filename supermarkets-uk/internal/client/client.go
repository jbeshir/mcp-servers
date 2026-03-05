// Package client orchestrates supermarket datasources with rate limiting and concurrent search.
package client

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/auth"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/osp"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/sainsburys"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/tesco"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/wrapper"
)

// SupermarketInfo describes a supported supermarket for the list tool.
type SupermarketInfo struct {
	ID   datasource.SupermarketID `json:"id"`
	Name string                   `json:"name"`
}

// Client orchestrates multiple supermarket datasources.
type Client struct {
	datasources map[datasource.SupermarketID]datasource.Datasource
	browser     *scraper.Browser
}

// Config holds configuration for creating a Client.
type Config struct {
	// Cookies holds pre-loaded session cookies per supermarket.
	Cookies map[datasource.SupermarketID][]*http.Cookie
	// NeedLogin is the set of supermarkets flagged for login that
	// don't yet have cached cookies. These get lazy-auth wrappers.
	NeedLogin map[datasource.SupermarketID]bool
	// Store persists cookies obtained via interactive login.
	Store *auth.CookieStore
}

// NewClient creates a new client with all four supermarket datasources.
// Supermarkets with cached cookies use them immediately. Supermarkets
// flagged for login but without cached cookies get a lazy-auth wrapper
// that triggers interactive login on first use.
func NewClient(cfg Config) *Client {
	browser := scraper.NewBrowser()

	sources := []datasource.AuthDatasource{
		tesco.NewDatasource(browser),
		sainsburys.NewDatasource(),
		osp.NewOcado(),
		osp.NewMorrisons(),
	}

	dsMap := make(
		map[datasource.SupermarketID]datasource.Datasource,
		len(sources),
	)
	for _, ds := range sources {
		id := ds.ID()

		// Inject cached cookies if available, then validate the session.
		if cookies := cfg.Cookies[id]; len(cookies) > 0 {
			ds.SetCookies(cookies)
			if !ds.CheckSession(context.Background()) {
				log.Printf("cached session for %s is invalid, clearing", id)
				ds.SetCookies(nil)
				if cfg.Store != nil {
					_ = cfg.Store.Clear(id)
				}
				cfg.NeedLogin[id] = true
			}
		}

		wrapped := datasource.Datasource(ds)

		if cfg.NeedLogin[id] && cfg.Store != nil {
			loginCfg, ok := auth.SupermarketLoginConfigs[id]
			if ok {
				store := cfg.Store
				smID := id
				doLogin := func(
					ctx context.Context,
				) ([]*http.Cookie, error) {
					c, err := auth.InteractiveLogin(
						ctx, smID, loginCfg,
					)
					if err != nil {
						return nil, err
					}
					if err := store.Save(smID, c); err != nil {
						log.Printf(
							"warning: failed to save "+
								"cookies for %s: %v",
							smID, err,
						)
					} else {
						log.Printf(
							"saved %d cookies for %s",
							len(c), smID,
						)
					}
					return c, nil
				}
				wrapped = wrapper.NewLazyAuth(ds, doLogin)
			}
		}

		dsMap[id] = wrapper.NewRateLimited(
			wrapped, rate.NewLimiter(1, 1),
		)
	}

	return &Client{
		datasources: dsMap,
		browser:     browser,
	}
}

// Close releases resources held by the client (e.g. headless browser).
func (c *Client) Close() {
	if c.browser != nil {
		c.browser.Close()
	}
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
		ds, ok := c.datasources[id]
		if !ok {
			results[i] = datasource.SearchResult{
				Supermarket: id,
				Error:       fmt.Sprintf("unknown supermarket: %s", id),
			}
			continue
		}

		wg.Add(1)
		go func(idx int, ds datasource.Datasource, sid datasource.SupermarketID) {
			defer wg.Done()

			products, err := ds.SearchProducts(ctx, query)
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

// GetDatasource returns a single datasource by ID.
func (c *Client) GetDatasource(id datasource.SupermarketID) (datasource.Datasource, bool) {
	ds, ok := c.datasources[id]
	return ds, ok
}

// ListSupermarkets returns info about all supported supermarkets.
func (c *Client) ListSupermarkets() []SupermarketInfo {
	var infos []SupermarketInfo
	for _, id := range datasource.AllSupermarkets {
		if ds, ok := c.datasources[id]; ok {
			infos = append(infos, SupermarketInfo{
				ID:   id,
				Name: ds.Name(),
			})
		}
	}
	return infos
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

// Package client orchestrates supermarket datasources with rate limiting and concurrent search.
package client

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/time/rate"

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

// NewClient creates a new client with all four supermarket datasources.
func NewClient(postcode string) *Client {
	browser := scraper.NewBrowser()

	sources := []datasource.Datasource{
		tesco.NewDatasource(browser),
		sainsburys.NewDatasource(),
		osp.NewOcado(),
		osp.NewMorrisons(),
	}

	dsMap := make(
		map[datasource.SupermarketID]datasource.Datasource, len(sources),
	)
	for _, ds := range sources {
		dsMap[ds.ID()] = wrapper.NewRateLimited(ds, rate.NewLimiter(1, 1))
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

// Package server provides the MCP server for Amazon product search.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jbeshir/mcp-servers/amazon-products/internal/scraper"
)

// Server is the MCP server for Amazon product search.
type Server struct {
	browser         *scraper.Browser
	defaultRegionID string
	mcpServer       *server.MCPServer
}

// NewServer creates a new MCP server.
func NewServer(browser *scraper.Browser, defaultRegionID string) *Server {
	s := &Server{browser: browser, defaultRegionID: defaultRegionID}
	s.mcpServer = server.NewMCPServer(
		"amazon",
		"0.1.0",
		server.WithLogging(),
	)
	s.registerTools()
	return s
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	regionDesc := fmt.Sprintf(
		"Amazon region ID (e.g. 'us', 'uk', 'de'). Defaults to %q. Use list_regions to see all available regions.",
		s.defaultRegionID,
	)

	s.mcpServer.AddTool(mcp.NewTool("search_products",
		mcp.WithDescription(
			"Search for products on Amazon. "+
				"Returns products with prices, ratings, and Prime eligibility."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search term (e.g. 'wireless headphones', 'running shoes')"),
		),
		mcp.WithString("region",
			mcp.Description(regionDesc),
		),
	), s.handleSearch)

	s.mcpServer.AddTool(mcp.NewTool("get_product_details",
		mcp.WithDescription(
			"Get detailed information about a specific Amazon product "+
				"including price, description, features, rating, and availability."),
		mcp.WithString("asin",
			mcp.Required(),
			mcp.Description("Amazon product ASIN (e.g. 'B09CDRVQZC')"),
		),
		mcp.WithString("region",
			mcp.Description(regionDesc),
		),
	), s.handleProductDetails)

	s.mcpServer.AddTool(mcp.NewTool("list_regions",
		mcp.WithDescription("List all supported Amazon regions with their IDs, names, and currencies."),
	), s.handleListRegions)
}

func (s *Server) resolveRegion(args map[string]any) (scraper.Region, error) {
	id := s.defaultRegionID
	if v, ok := args["region"].(string); ok && v != "" {
		id = v
	}
	region, ok := scraper.Regions[id]
	if !ok {
		var ids []string
		for k := range scraper.Regions {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		return scraper.Region{}, fmt.Errorf("unknown region %q; valid regions: %s",
			id, strings.Join(ids, ", "))
	}
	return region, nil
}

func (s *Server) handleSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, ok := request.Params.Arguments["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	region, err := s.resolveRegion(request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ds := scraper.NewDatasource(s.browser, region)
	products, err := ds.SearchProducts(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(formatError("search failed", err)), nil
	}

	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format results: %v", err)), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d product(s):\n\n%s", len(products), string(data)),
	), nil
}

func (s *Server) handleProductDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	asin, ok := request.Params.Arguments["asin"].(string)
	if !ok || asin == "" {
		return mcp.NewToolResultError("asin is required"), nil
	}

	region, err := s.resolveRegion(request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ds := scraper.NewDatasource(s.browser, region)
	product, err := ds.GetProductDetails(ctx, asin)
	if err != nil {
		return mcp.NewToolResultError(formatError("failed to get product", err)), nil
	}

	data, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format product: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

type regionInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Default  bool   `json:"default,omitempty"`
}

func (s *Server) handleListRegions(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var ids []string
	for id := range scraper.Regions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	regions := make([]regionInfo, 0, len(ids))
	for _, id := range ids {
		r := scraper.Regions[id]
		regions = append(regions, regionInfo{
			ID:       id,
			Name:     r.Name,
			Currency: r.Currency,
			Default:  id == s.defaultRegionID,
		})
	}

	data, err := json.MarshalIndent(regions, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format regions: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func formatError(prefix string, err error) string {
	if errors.Is(err, scraper.ErrCAPTCHA) || errors.Is(err, scraper.ErrBlocked) {
		return fmt.Sprintf("%s: %v — this is an anti-bot measure, retrying may work.", prefix, err)
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}

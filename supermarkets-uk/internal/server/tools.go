package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("search_products",
		mcp.WithDescription(
			"Search for grocery products across UK supermarkets. "+
				"Returns products with prices, availability, and promotions."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search term for products (e.g. 'milk', 'bread', 'chicken breast')"),
		),
		mcp.WithString("supermarkets",
			mcp.Description(
				"Comma-separated supermarket IDs to search. "+
					"Use list_supermarkets to see all available IDs. "+
					"Omit to search all."),
		),
	), s.handleSearchProducts)

	s.mcpServer.AddTool(mcp.NewTool("compare_prices",
		mcp.WithDescription(
			"Compare prices for a product across all UK supermarkets. "+
				"Searches all supermarkets and highlights the cheapest option."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Product to compare prices for (e.g. 'semi skimmed milk 2 pint')"),
		),
	), s.handleComparePrices)

	s.mcpServer.AddTool(mcp.NewTool("browse_categories",
		mcp.WithDescription("Browse product categories for a specific supermarket."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID. Use list_supermarkets to see all available IDs."),
		),
	), s.handleBrowseCategories)

	s.mcpServer.AddTool(mcp.NewTool("get_product_details",
		mcp.WithDescription(
			"Get detailed information about a specific product including price, "+
				"availability, description, ingredients, and nutritional information "+
				"where available."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID. Use list_supermarkets to see all available IDs."),
		),
		mcp.WithString("product_id",
			mcp.Required(),
			mcp.Description("Product ID from search results"),
		),
	), s.handleGetProductDetails)

	s.mcpServer.AddTool(mcp.NewTool("list_supermarkets",
		mcp.WithDescription("List all supported UK supermarkets with their IDs and status."),
	), s.handleListSupermarkets)
}

func (s *Server) handleSearchProducts(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	var supermarkets []datasource.SupermarketID
	if v, ok := args["supermarkets"].(string); ok && v != "" {
		supermarkets = client.ParseSupermarketIDs(v)
	}

	results := s.client.SearchAll(ctx, query, supermarkets)
	return formatSearchResults(results)
}

func (s *Server) handleComparePrices(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, ok := request.Params.Arguments["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	results := s.client.SearchAll(ctx, query, nil)
	return formatPriceComparison(query, results)
}

func (s *Server) handleBrowseCategories(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	supermarketID, ok := request.Params.Arguments["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	sid := datasource.SupermarketID(supermarketID)
	ds, ok := s.client.GetDatasource(sid)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown supermarket: %s", supermarketID)), nil
	}

	categories, err := ds.BrowseCategories(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to browse categories: %v", err)), nil
	}

	return formatCategories(categories)
}

func (s *Server) handleGetProductDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	supermarketID, ok := args["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	productID, ok := args["product_id"].(string)
	if !ok || productID == "" {
		return mcp.NewToolResultError("product_id is required"), nil
	}

	sid := datasource.SupermarketID(supermarketID)
	ds, ok := s.client.GetDatasource(sid)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown supermarket: %s", supermarketID)), nil
	}

	product, err := ds.GetProductDetails(ctx, productID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get product details: %v", err)), nil
	}

	return formatProduct(product)
}

func (s *Server) handleListSupermarkets(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	infos := s.client.ListSupermarkets()
	return formatSupermarkets(infos)
}

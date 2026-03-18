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
				"Returns products with prices, promotions, and availability. "+
				"The 'available' field is true/false when known, or omitted when unknown."),
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
				"description, ingredients, and nutritional information where available. "+
				"The 'available' field is true/false when known, or omitted when unknown."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID. Use list_supermarkets to see all available IDs."),
		),
		mcp.WithString("productId",
			mcp.Required(),
			mcp.Description("Product ID from search results"),
		),
	), s.handleGetProductDetails)

	s.mcpServer.AddTool(mcp.NewTool("get_order_history",
		mcp.WithDescription(
			"Get past grocery order history. Supported supermarkets: tesco. "+
				"Requires a logged-in session. "+
				"Returns orders with items, totals, delivery slots, and status."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID — must be 'tesco'."),
		),
		mcp.WithNumber("page",
			mcp.Description("Page number for pagination (default 1, 10 orders per page)."),
		),
	), s.handleGetOrderHistory)

	s.mcpServer.AddTool(mcp.NewTool("get_basket",
		mcp.WithDescription(
			"Get the current shopping basket contents. Supported supermarkets: tesco. "+
				"Requires a logged-in session. "+
				"Returns items with quantities, prices, and totals."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID — must be 'tesco'."),
		),
	), s.handleGetBasket)

	s.mcpServer.AddTool(mcp.NewTool("add_to_basket",
		mcp.WithDescription(
			"Add a product to the shopping basket or update its quantity. "+
				"Supported supermarkets: tesco. "+
				"Use product IDs from search results. "+
				"Requires a logged-in session."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID — must be 'tesco'."),
		),
		mcp.WithString("productId",
			mcp.Required(),
			mcp.Description("Product ID from search results"),
		),
		mcp.WithNumber("quantity",
			mcp.Description("Quantity to set (default 1). This is an absolute value, not relative."),
		),
	), s.handleAddToBasket)

	s.mcpServer.AddTool(mcp.NewTool("remove_from_basket",
		mcp.WithDescription(
			"Remove a product from the shopping basket. "+
				"Supported supermarkets: tesco. "+
				"Requires a logged-in session."),
		mcp.WithString("supermarket",
			mcp.Required(),
			mcp.Description("Supermarket ID — must be 'tesco'."),
		),
		mcp.WithString("productId",
			mcp.Required(),
			mcp.Description("Product ID to remove"),
		),
	), s.handleRemoveFromBasket)

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
	categories, err := s.client.BrowseCategories(ctx, sid)
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

	productID, ok := args["productId"].(string)
	if !ok || productID == "" {
		return mcp.NewToolResultError("productId is required"), nil
	}

	sid := datasource.SupermarketID(supermarketID)
	product, err := s.client.GetProductDetails(ctx, sid, productID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get product details: %v", err)), nil
	}

	return formatProduct(product)
}

func (s *Server) handleGetOrderHistory(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	supermarketID, ok := args["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	page := 1
	if v, ok := args["page"].(float64); ok && v > 0 {
		page = int(v)
	}

	sid := datasource.SupermarketID(supermarketID)
	result, err := s.client.GetOrderHistory(ctx, sid, page)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to get order history: %v", err),
		), nil
	}

	return formatOrderHistory(result)
}

func (s *Server) handleGetBasket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	supermarketID, ok := request.Params.Arguments["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	sid := datasource.SupermarketID(supermarketID)
	basket, err := s.client.GetBasket(ctx, sid)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to get basket: %v", err),
		), nil
	}

	return formatBasket(basket)
}

func (s *Server) handleAddToBasket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	supermarketID, ok := args["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	productID, ok := args["productId"].(string)
	if !ok || productID == "" {
		return mcp.NewToolResultError("productId is required"), nil
	}

	quantity := 1
	if v, ok := args["quantity"].(float64); ok && v > 0 {
		quantity = int(v)
	}

	sid := datasource.SupermarketID(supermarketID)
	basket, err := s.client.UpdateBasketItem(ctx, sid, productID, quantity)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to add to basket: %v", err),
		), nil
	}

	return formatBasket(basket)
}

func (s *Server) handleRemoveFromBasket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	supermarketID, ok := args["supermarket"].(string)
	if !ok || supermarketID == "" {
		return mcp.NewToolResultError("supermarket is required"), nil
	}

	productID, ok := args["productId"].(string)
	if !ok || productID == "" {
		return mcp.NewToolResultError("productId is required"), nil
	}

	sid := datasource.SupermarketID(supermarketID)
	basket, err := s.client.UpdateBasketItem(ctx, sid, productID, 0)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to remove from basket: %v", err),
		), nil
	}

	return formatBasket(basket)
}

func (s *Server) handleListSupermarkets(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	infos := s.client.ListSupermarkets()
	return formatSupermarkets(infos)
}

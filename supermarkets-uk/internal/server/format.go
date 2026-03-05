package server

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func formatSearchResults(results []datasource.SearchResult) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format results: %v", err)), nil
	}

	total := 0
	for _, r := range results {
		total += len(r.Products)
	}

	msg := fmt.Sprintf(
		"Found %d product(s) across %d supermarket(s):\n\n%s",
		total, len(results), string(data),
	)
	return mcp.NewToolResultText(msg), nil
}

func formatPriceComparison(query string, results []datasource.SearchResult) (*mcp.CallToolResult, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Price comparison for \"%s\":\n\n", query))

	cheapestPrice := math.MaxFloat64
	cheapestStore := ""
	cheapestProduct := ""

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("## %s\n", r.Supermarket))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n\n", r.Error))
			continue
		}
		if len(r.Products) == 0 {
			sb.WriteString("  No products found.\n\n")
			continue
		}
		for _, p := range r.Products {
			sb.WriteString(fmt.Sprintf("  - %s: £%.2f", p.Name, p.Price))
			if p.PricePerUnit != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", p.PricePerUnit))
			}
			if p.Promotion != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", p.Promotion))
			}
			sb.WriteString("\n")

			if p.Price > 0 && p.Price < cheapestPrice {
				cheapestPrice = p.Price
				cheapestStore = string(r.Supermarket)
				cheapestProduct = p.Name
			}
		}
		sb.WriteString("\n")
	}

	if cheapestStore != "" {
		sb.WriteString(fmt.Sprintf("**Cheapest:** %s at %s (£%.2f)\n", cheapestProduct, cheapestStore, cheapestPrice))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func formatCategories(categories []datasource.Category) (*mcp.CallToolResult, error) {
	if len(categories) == 0 {
		return mcp.NewToolResultText("No categories found."), nil
	}
	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format categories: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d category(ies):\n\n%s", len(categories), string(data))), nil
}

func formatProduct(product *datasource.Product) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format product: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatSupermarkets(infos []client.SupermarketInfo) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format supermarkets: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Supported supermarkets:\n\n%s", string(data))), nil
}

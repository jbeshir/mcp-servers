package server

import (
	"encoding/json"
	"fmt"

	"github.com/jbeshir/mcp-servers/wanikani/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatResource[T any](r *client.Resource[T]) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatCollection[T any](noun string, items []client.Resource[T], totalCount int) (*mcp.CallToolResult, error) {
	if len(items) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No %ss found.", noun)), nil
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format %ss: %v", noun, err)), nil
	}
	header := fmt.Sprintf("Showing %d of %d %s(s):\n\n", len(items), totalCount, noun)
	return mcp.NewToolResultText(header + string(data)), nil
}

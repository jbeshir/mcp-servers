package server

import (
	"encoding/json"
	"fmt"

	"github.com/jbeshir/mcp-servers/wanikani/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatResource[T any](r *client.Resource[T]) *mcp.CallToolResult {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err))
	}
	return mcp.NewToolResultText(string(data))
}

func formatCollection[T any](noun string, items []client.Resource[T], totalCount int) *mcp.CallToolResult {
	if len(items) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No %ss found.", noun))
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format %ss: %v", noun, err))
	}
	header := fmt.Sprintf("Showing %d of %d %s(s):\n\n", len(items), totalCount, noun)
	return mcp.NewToolResultText(header + string(data))
}

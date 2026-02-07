package server

import (
	"encoding/json"
	"fmt"

	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatNode(node *client.Node) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format node: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatNodes(nodes []client.Node) (*mcp.CallToolResult, error) {
	if len(nodes) == 0 {
		return mcp.NewToolResultText("No nodes found."), nil
	}
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format nodes: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d node(s):\n\n%s", len(nodes), string(data))), nil
}

// SearchResult is a node with its breadcrumb path.
type SearchResult struct {
	client.Node
	Path []string `json:"path"`
}

func formatSearchResults(results []SearchResult) (*mcp.CallToolResult, error) {
	if len(results) == 0 {
		return mcp.NewToolResultText("No matching nodes found."), nil
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format search results: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d result(s):\n\n%s", len(results), string(data))), nil
}

func formatTargets(targets []client.Target) (*mcp.CallToolResult, error) {
	if len(targets) == 0 {
		return mcp.NewToolResultText("No targets found."), nil
	}
	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format targets: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d target(s):\n\n%s", len(targets), string(data))), nil
}

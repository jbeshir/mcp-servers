package server

import (
	"context"
	"strings"

	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleSearchNodes(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = min(int(l), 200)
	}

	var filterCompleted *bool
	if c, ok := args["completed"].(bool); ok {
		filterCompleted = &c
	}

	nodes, err := s.cache.GetAllNodes(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed to fetch nodes: " + err.Error()), nil
	}

	// Build ID→Node index for parent lookups.
	index := make(map[string]*client.Node, len(nodes))
	for i := range nodes {
		index[nodes[i].ID] = &nodes[i]
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for i := range nodes {
		node := &nodes[i]

		// Filter by completion status if specified.
		if filterCompleted != nil {
			isCompleted := node.CompletedAt != nil
			if node.Completed != nil {
				isCompleted = *node.Completed
			}
			if isCompleted != *filterCompleted {
				continue
			}
		}

		// Case-insensitive substring match on name and note.
		nameLower := strings.ToLower(node.Name)
		noteLower := ""
		if node.Note != nil {
			noteLower = strings.ToLower(*node.Note)
		}

		if !strings.Contains(nameLower, queryLower) && !strings.Contains(noteLower, queryLower) {
			continue
		}

		// Build breadcrumb path.
		path := buildPath(node, index)

		results = append(results, SearchResult{
			Node: *node,
			Path: path,
		})

		if len(results) >= limit {
			break
		}
	}

	return formatSearchResults(results)
}

// buildPath walks the ParentID chain to build a breadcrumb trail of ancestor names.
func buildPath(node *client.Node, index map[string]*client.Node) []string {
	var path []string
	seen := make(map[string]bool) // Prevent infinite loops from circular references.

	current := node
	for current.ParentID != nil && *current.ParentID != "" {
		if seen[*current.ParentID] {
			break
		}
		seen[*current.ParentID] = true

		parent, ok := index[*current.ParentID]
		if !ok {
			break
		}
		path = append(path, parent.Name)
		current = parent
	}

	// Reverse to get root-first order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

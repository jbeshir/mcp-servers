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

	index := buildIndex(nodes)
	results := searchNodes(nodes, index, strings.ToLower(query), filterCompleted, limit)

	return formatSearchResults(results)
}

func buildIndex(nodes []client.Node) map[string]*client.Node {
	index := make(map[string]*client.Node, len(nodes))
	for i := range nodes {
		index[nodes[i].ID] = &nodes[i]
	}
	return index
}

func searchNodes(
	nodes []client.Node, index map[string]*client.Node,
	queryLower string, filterCompleted *bool, limit int,
) []SearchResult {
	var results []SearchResult

	for i := range nodes {
		node := &nodes[i]

		if !matchesFilter(node, filterCompleted) {
			continue
		}

		if !matchesQuery(node, queryLower) {
			continue
		}

		results = append(results, SearchResult{
			Node: *node,
			Path: buildPath(node, index),
		})

		if len(results) >= limit {
			break
		}
	}

	return results
}

func matchesFilter(node *client.Node, filterCompleted *bool) bool {
	if filterCompleted == nil {
		return true
	}
	isCompleted := node.CompletedAt != nil
	if node.Completed != nil {
		isCompleted = *node.Completed
	}
	return isCompleted == *filterCompleted
}

func matchesQuery(node *client.Node, queryLower string) bool {
	if strings.Contains(strings.ToLower(node.Name), queryLower) {
		return true
	}
	if node.Note != nil && strings.Contains(strings.ToLower(*node.Note), queryLower) {
		return true
	}
	return false
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

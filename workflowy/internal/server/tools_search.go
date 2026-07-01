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
	args := request.GetArguments()

	query, _ := args["query"].(string)

	var completedAfter, completedBefore *int64
	if v, ok := args["completed_after"].(float64); ok {
		i := int64(v)
		completedAfter = &i
	}
	if v, ok := args["completed_before"].(float64); ok {
		i := int64(v)
		completedBefore = &i
	}

	if !validateSearchArgs(query, completedAfter, completedBefore) {
		return mcp.NewToolResultError("query or a completed_after/completed_before bound is required"), nil
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
	results := searchNodes(nodes, index, strings.ToLower(query), filterCompleted, completedAfter, completedBefore, limit)

	return formatSearchResults(results)
}

func validateSearchArgs(query string, completedAfter, completedBefore *int64) bool {
	return query != "" || completedAfter != nil || completedBefore != nil
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
	queryLower string, filterCompleted *bool,
	completedAfter, completedBefore *int64,
	limit int,
) []SearchResult {
	var results []SearchResult
	completedMemo := make(map[string]bool)
	dateBoundsPresent := completedAfter != nil || completedBefore != nil

	for i := range nodes {
		node := &nodes[i]

		if !matchesFilter(node, index, completedMemo, filterCompleted, dateBoundsPresent) {
			continue
		}

		if queryLower != "" && !matchesQuery(node, queryLower) {
			continue
		}

		if dateBoundsPresent && !matchesDateRange(node, completedAfter, completedBefore) {
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

func matchesFilter(
	node *client.Node, index map[string]*client.Node,
	memo map[string]bool, filterCompleted *bool, dateBoundsPresent bool,
) bool {
	completed := isEffectivelyCompleted(node, index, memo)
	if filterCompleted == nil {
		if dateBoundsPresent {
			// Date bounds imply interest in completed items; matchesDateRange handles the actual gate.
			return true
		}
		// Default: exclude completed items.
		return !completed
	}
	return completed == *filterCompleted
}

func matchesDateRange(node *client.Node, after, before *int64) bool {
	if node.CompletedAt == nil {
		return false
	}
	ts := *node.CompletedAt
	if after != nil && ts < *after {
		return false
	}
	if before != nil && ts > *before {
		return false
	}
	return true
}

// isEffectivelyCompleted returns true if this node or any ancestor is completed.
// Results are memoized so repeated ancestor lookups are O(1).
func isEffectivelyCompleted(
	node *client.Node, index map[string]*client.Node,
	memo map[string]bool,
) bool {
	if v, ok := memo[node.ID]; ok {
		return v
	}

	completed := nodeIsCompleted(node)
	if !completed && node.ParentID != nil && *node.ParentID != "" {
		if parent, ok := index[*node.ParentID]; ok {
			completed = isEffectivelyCompleted(parent, index, memo)
		}
	}

	memo[node.ID] = completed
	return completed
}

func nodeIsCompleted(node *client.Node) bool {
	if node.Completed != nil {
		return *node.Completed
	}
	return node.CompletedAt != nil && *node.CompletedAt != 0
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

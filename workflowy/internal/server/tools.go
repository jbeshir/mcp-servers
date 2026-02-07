package server

import (
	"context"
	"fmt"

	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGetNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	nodeID, ok := request.Params.Arguments["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	node, err := s.client.GetNode(ctx, nodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get node: %v", err)), nil
	}

	return formatNode(node)
}

func (s *Server) handleListChildren(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	parentID, _ := request.Params.Arguments["parent_id"].(string)

	nodes, err := s.client.ListChildren(ctx, parentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list children: %v", err)), nil
	}

	return formatNodes(nodes)
}

func (s *Server) handleCreateNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	req := client.CreateNodeRequest{
		Name: name,
	}
	if parentID, ok := args["parent_id"].(string); ok && parentID != "" {
		req.ParentID = parentID
	}
	if note, ok := args["note"].(string); ok {
		req.Note = note
	}
	if layoutMode, ok := args["layout_mode"].(string); ok {
		req.LayoutMode = layoutMode
	}
	if position, ok := args["position"].(string); ok {
		req.Position = position
	}

	result, err := s.client.CreateNode(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Created node with ID: %s", result.ItemID)), nil
}

func (s *Server) handleUpdateNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	nodeID, ok := args["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	req := client.UpdateNodeRequest{}
	if name, ok := args["name"].(string); ok {
		req.Name = &name
	}
	if note, ok := args["note"].(string); ok {
		req.Note = &note
	}
	if layoutMode, ok := args["layout_mode"].(string); ok {
		req.LayoutMode = &layoutMode
	}

	if err := s.client.UpdateNode(ctx, nodeID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Updated node %s", nodeID)), nil
}

func (s *Server) handleDeleteNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	nodeID, ok := request.Params.Arguments["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	if err := s.client.DeleteNode(ctx, nodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Deleted node %s", nodeID)), nil
}

func (s *Server) handleMoveNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	nodeID, ok := args["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	req := client.MoveNodeRequest{}
	if parentID, ok := args["parent_id"].(string); ok {
		req.ParentID = parentID
	}
	if position, ok := args["position"].(string); ok {
		req.Position = position
	}

	if err := s.client.MoveNode(ctx, nodeID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to move node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Moved node %s", nodeID)), nil
}

func (s *Server) handleCompleteNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	nodeID, ok := request.Params.Arguments["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	if err := s.client.CompleteNode(ctx, nodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to complete node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Completed node %s", nodeID)), nil
}

func (s *Server) handleUncompleteNode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	nodeID, ok := request.Params.Arguments["node_id"].(string)
	if !ok || nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	if err := s.client.UncompleteNode(ctx, nodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to uncomplete node: %v", err)), nil
	}

	s.cache.Invalidate()
	return mcp.NewToolResultText(fmt.Sprintf("Uncompleted node %s", nodeID)), nil
}

func (s *Server) handleListTargets(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	targets, err := s.client.ListTargets(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list targets: %v", err)), nil
	}

	return formatTargets(targets)
}

package server

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
)

// setOptionalString sets a query parameter from args if the string key is present and non-empty.
func setOptionalString(params url.Values, args map[string]any, key string) {
	if v, ok := args[key].(string); ok && v != "" {
		params.Set(key, v)
	}
}

// setOptionalLimit sets a "limit" query parameter from args if present and positive.
func setOptionalLimit(params url.Values, args map[string]any) {
	if v, ok := args["limit"].(float64); ok && v > 0 {
		params.Set("limit", strconv.Itoa(int(v)))
	}
}

// setOptionalNumber sets a query parameter from args if the numeric key is present and positive.
func setOptionalNumber(params url.Values, args map[string]any, key string) {
	if v, ok := args[key].(float64); ok && v > 0 {
		params.Set(key, strconv.Itoa(int(v)))
	}
}

func (s *Server) handleSearchMarkets(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "term")
	setOptionalString(params, args, "sort")
	setOptionalString(params, args, "filter")
	setOptionalString(params, args, "contractType")
	setOptionalString(params, args, "topicSlug")
	setOptionalLimit(params, args)

	markets, err := s.client.SearchMarkets(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search markets: %v", err)), nil
	}

	return formatMarkets(markets)
}

func (s *Server) handleGetMarket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	marketID, ok := request.Params.Arguments["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	market, err := s.client.GetMarket(ctx, marketID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get market: %v", err)), nil
	}

	return formatFullMarket(market)
}

func (s *Server) handleGetUser(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	username, ok := request.Params.Arguments["username"].(string)
	if !ok || username == "" {
		return mcp.NewToolResultError("username is required"), nil
	}

	user, err := s.client.GetUser(ctx, username)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get user: %v", err)), nil
	}

	return formatUser(user)
}

func (s *Server) handleGetMe(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	user, err := s.client.GetMe(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get authenticated user: %v", err)), nil
	}

	return formatUser(user)
}

func (s *Server) handleListBets(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "userId")
	setOptionalString(params, args, "contractId")
	setOptionalLimit(params, args)
	setOptionalString(params, args, "before")
	setOptionalString(params, args, "kinds")

	bets, err := s.client.ListBets(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list bets: %v", err)), nil
	}

	return formatBets(bets)
}

func (s *Server) handleGetComments(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "contractId")
	setOptionalLimit(params, args)
	setOptionalString(params, args, "userId")

	comments, err := s.client.GetComments(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get comments: %v", err)), nil
	}

	return formatComments(comments)
}

func (s *Server) handleGetPositions(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	marketID, ok := args["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	params := url.Values{}
	setOptionalString(params, args, "order")
	setOptionalNumber(params, args, "top")
	setOptionalNumber(params, args, "bottom")
	setOptionalString(params, args, "userId")

	positions, err := s.client.GetPositions(ctx, marketID, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get positions: %v", err)), nil
	}

	return formatPositions(positions)
}

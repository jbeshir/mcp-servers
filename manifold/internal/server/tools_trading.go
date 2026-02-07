package server

import (
	"context"
	"fmt"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handlePlaceBet(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	amount, ok := args["amount"].(float64)
	if !ok || amount <= 0 {
		return mcp.NewToolResultError("amount is required and must be positive"), nil
	}

	contractID, ok := args["contractId"].(string)
	if !ok || contractID == "" {
		return mcp.NewToolResultError("contractId is required"), nil
	}

	req := client.PlaceBetRequest{
		Amount:     amount,
		ContractID: contractID,
	}
	if outcome, ok := args["outcome"].(string); ok && outcome != "" {
		req.Outcome = outcome
	}
	if limitProb, ok := args["limitProb"].(float64); ok {
		req.LimitProb = &limitProb
	}
	if expiresAt, ok := args["expiresAt"].(float64); ok {
		v := int64(expiresAt)
		req.ExpiresAt = &v
	}
	if dryRun, ok := args["dryRun"].(bool); ok {
		req.DryRun = &dryRun
	}

	bet, err := s.client.PlaceBet(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to place bet: %v", err)), nil
	}

	return formatBet(bet)
}

func (s *Server) handleSellShares(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	marketID, ok := args["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	req := client.SellSharesRequest{}
	if outcome, ok := args["outcome"].(string); ok && outcome != "" {
		req.Outcome = outcome
	}
	if shares, ok := args["shares"].(float64); ok {
		req.Shares = &shares
	}
	if answerID, ok := args["answerId"].(string); ok && answerID != "" {
		req.AnswerID = answerID
	}

	bet, err := s.client.SellShares(ctx, marketID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sell shares: %v", err)), nil
	}

	return formatBet(bet)
}

func (s *Server) handleCancelBet(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	betID, ok := request.Params.Arguments["bet_id"].(string)
	if !ok || betID == "" {
		return mcp.NewToolResultError("bet_id is required"), nil
	}

	if err := s.client.CancelBet(ctx, betID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to cancel bet: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Cancelled bet %s", betID)), nil
}

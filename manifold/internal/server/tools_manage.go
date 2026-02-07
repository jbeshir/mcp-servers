package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func buildCreateMarketRequest(
	args map[string]any, outcomeType, question string,
) client.CreateMarketRequest {
	req := client.CreateMarketRequest{
		OutcomeType: outcomeType,
		Question:    question,
	}
	if description, ok := args["description"].(string); ok && description != "" {
		req.Description = description
	}
	if closeTime, ok := args["closeTime"].(float64); ok {
		v := int64(closeTime)
		req.CloseTime = &v
	}
	if initialProb, ok := args["initialProb"].(float64); ok {
		req.InitialProb = &initialProb
	}
	if minVal, ok := args["min"].(float64); ok {
		req.Min = &minVal
	}
	if maxVal, ok := args["max"].(float64); ok {
		req.Max = &maxVal
	}
	if isLogScale, ok := args["isLogScale"].(bool); ok {
		req.IsLogScale = &isLogScale
	}
	if answers, ok := args["answers"].(string); ok && answers != "" {
		parts := strings.Split(answers, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		req.Answers = parts
	}
	return req
}

func (s *Server) handleCreateMarket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	outcomeType, ok := args["outcomeType"].(string)
	if !ok || outcomeType == "" {
		return mcp.NewToolResultError("outcomeType is required"), nil
	}

	question, ok := args["question"].(string)
	if !ok || question == "" {
		return mcp.NewToolResultError("question is required"), nil
	}

	req := buildCreateMarketRequest(args, outcomeType, question)

	market, err := s.client.CreateMarket(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create market: %v", err)), nil
	}

	return formatLiteMarket(market)
}

func (s *Server) handleResolveMarket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	marketID, ok := args["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	outcome, ok := args["outcome"].(string)
	if !ok || outcome == "" {
		return mcp.NewToolResultError("outcome is required"), nil
	}

	req := client.ResolveMarketRequest{
		Outcome: outcome,
	}
	if value, ok := args["value"].(float64); ok {
		req.Value = &value
	}
	if probabilityInt, ok := args["probabilityInt"].(float64); ok {
		v := int(probabilityInt)
		req.ProbabilityInt = &v
	}
	if answerID, ok := args["answerId"].(string); ok && answerID != "" {
		req.AnswerID = answerID
	}

	if err := s.client.ResolveMarket(ctx, marketID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve market: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Resolved market %s to %s", marketID, outcome)), nil
}

func (s *Server) handleCloseMarket(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	marketID, ok := args["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	req := client.CloseMarketRequest{}
	if closeTime, ok := args["closeTime"].(float64); ok {
		v := int64(closeTime)
		req.CloseTime = &v
	}

	if err := s.client.CloseMarket(ctx, marketID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to close market: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Closed market %s", marketID)), nil
}

func (s *Server) handleAddComment(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	contractID, ok := args["contractId"].(string)
	if !ok || contractID == "" {
		return mcp.NewToolResultError("contractId is required"), nil
	}

	markdown, ok := args["markdown"].(string)
	if !ok || markdown == "" {
		return mcp.NewToolResultError("markdown is required"), nil
	}

	req := client.AddCommentRequest{
		ContractID: contractID,
		Markdown:   markdown,
	}

	comment, err := s.client.AddComment(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add comment: %v", err)), nil
	}

	return formatComment(comment)
}

func (s *Server) handleAddLiquidity(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	marketID, ok := args["market_id"].(string)
	if !ok || marketID == "" {
		return mcp.NewToolResultError("market_id is required"), nil
	}

	amount, ok := args["amount"].(float64)
	if !ok || amount <= 0 {
		return mcp.NewToolResultError("amount is required and must be positive"), nil
	}

	req := client.AddLiquidityRequest{
		Amount: amount,
	}

	if err := s.client.AddLiquidity(ctx, marketID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add liquidity: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added %.0f mana liquidity to market %s", amount, marketID)), nil
}

func (s *Server) handleSendMana(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	toIDsStr, ok := args["toIds"].(string)
	if !ok || toIDsStr == "" {
		return mcp.NewToolResultError("toIds is required"), nil
	}

	amount, ok := args["amount"].(float64)
	if !ok || amount <= 0 {
		return mcp.NewToolResultError("amount is required and must be positive"), nil
	}

	toIDs := strings.Split(toIDsStr, ",")
	for i := range toIDs {
		toIDs[i] = strings.TrimSpace(toIDs[i])
	}

	req := client.SendManaRequest{
		ToIDs:  toIDs,
		Amount: amount,
	}
	if message, ok := args["message"].(string); ok && message != "" {
		req.Message = message
	}

	if err := s.client.SendMana(ctx, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send mana: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Sent %.0f mana to %d user(s)", amount, len(toIDs))), nil
}

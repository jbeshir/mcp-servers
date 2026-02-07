package server

import (
	"encoding/json"
	"fmt"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatMarkets(markets []client.LiteMarket) (*mcp.CallToolResult, error) {
	if len(markets) == 0 {
		return mcp.NewToolResultText("No markets found."), nil
	}
	data, err := json.MarshalIndent(markets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format markets: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d market(s):\n\n%s", len(markets), string(data))), nil
}

func formatLiteMarket(market *client.LiteMarket) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(market, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format market: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatFullMarket(market *client.FullMarket) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(market, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format market: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatUser(user *client.User) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format user: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatBets(bets []client.Bet) (*mcp.CallToolResult, error) {
	if len(bets) == 0 {
		return mcp.NewToolResultText("No bets found."), nil
	}
	data, err := json.MarshalIndent(bets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format bets: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d bet(s):\n\n%s", len(bets), string(data))), nil
}

func formatBet(bet *client.Bet) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(bet, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format bet: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatComments(comments []client.Comment) (*mcp.CallToolResult, error) {
	if len(comments) == 0 {
		return mcp.NewToolResultText("No comments found."), nil
	}
	data, err := json.MarshalIndent(comments, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format comments: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d comment(s):\n\n%s", len(comments), string(data))), nil
}

func formatComment(comment *client.Comment) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(comment, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format comment: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func formatPositions(positions []client.ContractMetric) (*mcp.CallToolResult, error) {
	if len(positions) == 0 {
		return mcp.NewToolResultText("No positions found."), nil
	}
	data, err := json.MarshalIndent(positions, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format positions: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d position(s):\n\n%s", len(positions), string(data))), nil
}

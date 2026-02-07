package server

import (
	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for Manifold Markets.
type Server struct {
	client    *client.Client
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server with the given client.
func NewServer(apiClient *client.Client) *Server {
	s := &Server{
		client: apiClient,
	}

	s.mcpServer = server.NewMCPServer(
		"manifold",
		"1.0.0",
		server.WithLogging(),
	)

	s.registerTools()

	return s
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("search_markets",
		mcp.WithDescription(
			"Search Manifold Markets by keyword and filters. "+
				"Returns a list of markets matching the criteria."),
		mcp.WithString("term",
			mcp.Description("Search query term to match in market questions"),
		),
		mcp.WithString("sort",
			mcp.Description(
				"Sort order: score, newest, resolve-date, close-date, "+
					"liquidity, last-updated, last-bet-time, "+
					"last-comment-time, most-popular, daily-score"),
		),
		mcp.WithString("filter",
			mcp.Description("Filter by status: all, open, closed, resolved"),
		),
		mcp.WithString("contractType",
			mcp.Description("Filter by type: ALL, BINARY, MULTIPLE_CHOICE, FREE_RESPONSE, PSEUDO_NUMERIC, BOUNTY, POLL, NUMBER"),
		),
		mcp.WithString("topicSlug",
			mcp.Description("Filter by topic/group slug"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (default: 100, max: 1000)"),
		),
	), s.handleSearchMarkets)

	s.mcpServer.AddTool(mcp.NewTool("get_market",
		mcp.WithDescription("Get full details of a specific Manifold market including answers and description."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID or slug"),
		),
	), s.handleGetMarket)

	s.mcpServer.AddTool(mcp.NewTool("get_user",
		mcp.WithDescription("Get a Manifold user's profile by username."),
		mcp.WithString("username",
			mcp.Required(),
			mcp.Description("The username to look up"),
		),
	), s.handleGetUser)

	s.mcpServer.AddTool(mcp.NewTool("get_me",
		mcp.WithDescription("Get the authenticated user's own Manifold profile."),
	), s.handleGetMe)

	s.mcpServer.AddTool(mcp.NewTool("list_bets",
		mcp.WithDescription("List bets with optional filters."),
		mcp.WithString("userId",
			mcp.Description("Filter by user ID"),
		),
		mcp.WithString("contractId",
			mcp.Description("Filter by market/contract ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of bets to return (default: 1000)"),
		),
		mcp.WithString("before",
			mcp.Description("Return bets before this bet ID (for pagination)"),
		),
		mcp.WithString("kinds",
			mcp.Description("Comma-separated bet kinds to include"),
		),
	), s.handleListBets)

	s.mcpServer.AddTool(mcp.NewTool("get_comments",
		mcp.WithDescription("Get comments on Manifold markets."),
		mcp.WithString("contractId",
			mcp.Description("Filter by market/contract ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of comments to return"),
		),
		mcp.WithString("userId",
			mcp.Description("Filter by user ID"),
		),
	), s.handleGetComments)

	s.mcpServer.AddTool(mcp.NewTool("get_positions",
		mcp.WithDescription("Get user positions (holdings) for a specific market."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID"),
		),
		mcp.WithString("order",
			mcp.Description("Sort order: profit or shares"),
		),
		mcp.WithNumber("top",
			mcp.Description("Number of top positions to return"),
		),
		mcp.WithNumber("bottom",
			mcp.Description("Number of bottom positions to return"),
		),
		mcp.WithString("userId",
			mcp.Description("Filter to a specific user's position"),
		),
	), s.handleGetPositions)

	s.mcpServer.AddTool(mcp.NewTool("place_bet",
		mcp.WithDescription(
			"Place a bet or limit order on a Manifold market. "+
				"Use dryRun=true to simulate without executing."),
		mcp.WithNumber("amount",
			mcp.Required(),
			mcp.Description("Amount of mana to bet"),
		),
		mcp.WithString("contractId",
			mcp.Required(),
			mcp.Description("The market/contract ID to bet on"),
		),
		mcp.WithString("outcome",
			mcp.Description("Outcome to bet on: YES or NO (for binary markets), or answer ID (for multiple choice)"),
		),
		mcp.WithNumber("limitProb",
			mcp.Description("Limit order probability (0.01-0.99). If set, creates a limit order instead of a market order"),
		),
		mcp.WithNumber("expiresAt",
			mcp.Description("Unix timestamp in milliseconds when the limit order expires"),
		),
		mcp.WithBoolean("dryRun",
			mcp.Description("If true, simulates the bet without executing it"),
		),
	), s.handlePlaceBet)

	s.mcpServer.AddTool(mcp.NewTool("sell_shares",
		mcp.WithDescription("Sell shares in a Manifold market."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID to sell shares in"),
		),
		mcp.WithString("outcome",
			mcp.Description("Which outcome's shares to sell: YES or NO"),
		),
		mcp.WithNumber("shares",
			mcp.Description("Number of shares to sell (omit to sell all)"),
		),
		mcp.WithString("answerId",
			mcp.Description("Answer ID for multiple choice markets"),
		),
	), s.handleSellShares)

	s.mcpServer.AddTool(mcp.NewTool("cancel_bet",
		mcp.WithDescription("Cancel a pending limit order."),
		mcp.WithString("bet_id",
			mcp.Required(),
			mcp.Description("The bet/limit order ID to cancel"),
		),
	), s.handleCancelBet)

	s.mcpServer.AddTool(mcp.NewTool("create_market",
		mcp.WithDescription(
			"Create a new Manifold market. "+
				"For BINARY markets, set initialProb. "+
				"For MULTIPLE_CHOICE, provide comma-separated answers. "+
				"For PSEUDO_NUMERIC, set min, max, and optionally isLogScale."),
		mcp.WithString("outcomeType",
			mcp.Required(),
			mcp.Description("Market type: BINARY, MULTIPLE_CHOICE, FREE_RESPONSE, PSEUDO_NUMERIC, BOUNTY, POLL, NUMBER"),
		),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The market question"),
		),
		mcp.WithString("description",
			mcp.Description("Market description (markdown)"),
		),
		mcp.WithNumber("closeTime",
			mcp.Description("Unix timestamp in milliseconds when the market closes"),
		),
		mcp.WithNumber("initialProb",
			mcp.Description("Initial probability for BINARY markets (1-99)"),
		),
		mcp.WithNumber("min",
			mcp.Description("Minimum value for PSEUDO_NUMERIC markets"),
		),
		mcp.WithNumber("max",
			mcp.Description("Maximum value for PSEUDO_NUMERIC markets"),
		),
		mcp.WithBoolean("isLogScale",
			mcp.Description("Use logarithmic scale for PSEUDO_NUMERIC markets"),
		),
		mcp.WithString("answers",
			mcp.Description("Comma-separated list of answers for MULTIPLE_CHOICE markets"),
		),
	), s.handleCreateMarket)

	s.mcpServer.AddTool(mcp.NewTool("resolve_market",
		mcp.WithDescription("Resolve a Manifold market you created."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID to resolve"),
		),
		mcp.WithString("outcome",
			mcp.Required(),
			mcp.Description("Resolution: YES, NO, MKT, CANCEL, or answer ID for multiple choice"),
		),
		mcp.WithNumber("value",
			mcp.Description("Resolution value for PSEUDO_NUMERIC markets"),
		),
		mcp.WithNumber("probabilityInt",
			mcp.Description("Probability (0-100) for MKT resolution of BINARY markets"),
		),
		mcp.WithString("answerId",
			mcp.Description("Answer ID for resolving MULTIPLE_CHOICE markets"),
		),
	), s.handleResolveMarket)

	s.mcpServer.AddTool(mcp.NewTool("close_market",
		mcp.WithDescription("Close a Manifold market (set or change its closing time)."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID to close"),
		),
		mcp.WithNumber("closeTime",
			mcp.Description("New closing time as Unix timestamp in milliseconds (omit to close immediately)"),
		),
	), s.handleCloseMarket)

	s.mcpServer.AddTool(mcp.NewTool("add_comment",
		mcp.WithDescription("Add a comment to a Manifold market."),
		mcp.WithString("contractId",
			mcp.Required(),
			mcp.Description("The market/contract ID to comment on"),
		),
		mcp.WithString("markdown",
			mcp.Required(),
			mcp.Description("Comment content in markdown format"),
		),
	), s.handleAddComment)

	s.mcpServer.AddTool(mcp.NewTool("add_liquidity",
		mcp.WithDescription("Add mana liquidity to a Manifold market's pool."),
		mcp.WithString("market_id",
			mcp.Required(),
			mcp.Description("The market ID to add liquidity to"),
		),
		mcp.WithNumber("amount",
			mcp.Required(),
			mcp.Description("Amount of mana to add as liquidity"),
		),
	), s.handleAddLiquidity)

	s.mcpServer.AddTool(mcp.NewTool("send_mana",
		mcp.WithDescription("Send mana to one or more Manifold users."),
		mcp.WithString("toIds",
			mcp.Required(),
			mcp.Description("Comma-separated list of user IDs to send mana to"),
		),
		mcp.WithNumber("amount",
			mcp.Required(),
			mcp.Description("Amount of mana to send to each user"),
		),
		mcp.WithString("message",
			mcp.Description("Optional message to include with the mana transfer"),
		),
	), s.handleSendMana)
}

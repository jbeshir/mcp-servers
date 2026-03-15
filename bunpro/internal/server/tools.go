package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_user",
		mcp.WithDescription(
			"Get the authenticated Bunpro user's profile, "+
				"including level, XP, streak, and subscription status."),
	), s.handleGetUser)

	s.mcpServer.AddTool(mcp.NewTool("get_study_queue",
		mcp.WithDescription("Get the number of grammar and vocabulary reviews currently due."),
	), s.handleGetStudyQueue)

	s.mcpServer.AddTool(mcp.NewTool("get_decks",
		mcp.WithDescription("Get the user's study deck settings, including daily goals, progress counts, and batch sizes."),
	), s.handleGetDecks)

	s.mcpServer.AddTool(mcp.NewTool("get_stats",
		mcp.WithDescription(
			"Get base statistics: days studied, streak, items studied, "+
				"last session accuracy, and earned badges."),
	), s.handleGetStats)

	s.mcpServer.AddTool(mcp.NewTool("get_jlpt_progress",
		mcp.WithDescription(
			"Get JLPT progress across all levels (N5-N1), "+
				"showing SRS stage counts for grammar and vocabulary."),
	), s.handleGetJLPTProgress)

	s.mcpServer.AddTool(mcp.NewTool("get_review_forecast",
		mcp.WithDescription("Get a forecast of upcoming reviews, split by grammar and vocabulary."),
		mcp.WithString("granularity",
			mcp.Description("Forecast granularity: \"daily\" (default) or \"hourly\""),
		),
	), s.handleGetReviewForecast)

	s.mcpServer.AddTool(mcp.NewTool("get_srs_overview",
		mcp.WithDescription(
			"Get aggregate SRS level counts (beginner through master, "+
				"plus ghost and self-study) for grammar and vocabulary."),
	), s.handleGetSRSOverview)

	s.mcpServer.AddTool(mcp.NewTool("get_review_activity",
		mcp.WithDescription("Get daily review counts for the past ~30 days, split by grammar and vocabulary."),
	), s.handleGetReviewActivity)

	s.mcpServer.AddTool(mcp.NewTool("get_grammar_srs_details",
		mcp.WithDescription(
			"Get paginated grammar review items at a specific SRS level. "+
				"Use lookup_id with get_grammar_point for full details."),
		mcp.WithString("level",
			mcp.Required(),
			mcp.Description("SRS level: beginner, adept, seasoned, expert, or master"),
		),
		mcp.WithNumber("page",
			mcp.Description("Page number (default: 1)"),
		),
	), s.handleGetGrammarSRSDetails)

	s.mcpServer.AddTool(mcp.NewTool("get_vocab_srs_details",
		mcp.WithDescription(
			"Get paginated vocabulary review items at a specific SRS level. "+
				"Use lookup_id with get_vocab for full details."),
		mcp.WithString("level",
			mcp.Required(),
			mcp.Description("SRS level: beginner, adept, seasoned, expert, or master"),
		),
		mcp.WithNumber("page",
			mcp.Description("Page number (default: 1)"),
		),
	), s.handleGetVocabSRSDetails)

	s.mcpServer.AddTool(mcp.NewTool("get_grammar_point",
		mcp.WithDescription(
			"Get details of a specific grammar point by ID, "+
				"including meaning, structure, nuance, and study questions."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Grammar point ID"),
		),
	), s.handleGetGrammarPoint)

	s.mcpServer.AddTool(mcp.NewTool("get_vocab",
		mcp.WithDescription(
			"Get details of a vocabulary item by slug or ID, "+
				"including readings, pitch accent, frequency data, "+
				"and JMDict entries."),
		mcp.WithString("slugOrId",
			mcp.Required(),
			mcp.Description("Vocabulary slug (e.g. \"食べる\") or numeric ID"),
		),
	), s.handleGetVocab)
}

func (s *Server) handleGetUser(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	user, err := s.client.GetUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get user: %v", err)), nil
	}
	return formatJSON(user)
}

func (s *Server) handleGetStudyQueue(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	queue, err := s.client.GetStudyQueue(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get study queue: %v", err)), nil
	}
	return formatJSON(queue)
}

func (s *Server) handleGetDecks(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	decks, err := s.client.GetDecks(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get decks: %v", err)), nil
	}
	return formatJSON(decks)
}

func (s *Server) handleGetStats(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	stats, err := s.client.GetBaseStats(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get stats: %v", err)), nil
	}
	return formatJSON(stats)
}

func (s *Server) handleGetJLPTProgress(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	progress, err := s.client.GetJLPTProgress(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get JLPT progress: %v", err)), nil
	}
	return formatJSON(progress)
}

func (s *Server) handleGetReviewForecast(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	granularity, _ := request.Params.Arguments["granularity"].(string)

	var err error
	var result any
	if granularity == "hourly" {
		result, err = s.client.GetForecastHourly(ctx)
	} else {
		result, err = s.client.GetForecastDaily(ctx)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get review forecast: %v", err)), nil
	}
	return formatJSON(result)
}

func (s *Server) handleGetSRSOverview(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	overview, err := s.client.GetSRSOverview(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get SRS overview: %v", err)), nil
	}
	return formatJSON(overview)
}

func (s *Server) handleGetReviewActivity(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	activity, err := s.client.GetReviewActivity(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get review activity: %v", err)), nil
	}
	return formatJSON(activity)
}

func (s *Server) handleGetGrammarSRSDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleSRSDetails(ctx, request, "GrammarPoint")
}

func (s *Server) handleGetVocabSRSDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleSRSDetails(ctx, request, "Vocab")
}

func (s *Server) handleSRSDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
	reviewableType string,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	level, ok := args["level"].(string)
	if !ok || level == "" {
		return mcp.NewToolResultError("level is required"), nil
	}

	page := 1
	if v, ok := args["page"].(float64); ok && v > 0 {
		page = int(v)
	}

	details, err := s.client.GetSRSLevelDetails(ctx, level, reviewableType, page)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get SRS details: %v", err)), nil
	}
	if len(details.Reviews.Data) == 0 {
		return mcp.NewToolResultText("No items at this SRS level."), nil
	}
	return formatSRSDetails(details)
}

func (s *Server) handleGetGrammarPoint(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, ok := request.Params.Arguments["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	gp, err := s.client.GetGrammarPoint(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get grammar point: %v", err)), nil
	}
	return formatGrammarPoint(gp)
}

func (s *Server) handleGetVocab(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	slugOrID, ok := request.Params.Arguments["slugOrId"].(string)
	if !ok || slugOrID == "" {
		return mcp.NewToolResultError("slugOrId is required"), nil
	}

	vocab, err := s.client.GetVocab(ctx, slugOrID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get vocab: %v", err)), nil
	}
	return formatVocab(vocab)
}

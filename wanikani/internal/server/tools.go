package server

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_user",
		mcp.WithDescription(
			"Get the authenticated WaniKani user's profile, "+
				"including level, subscription, and vacation status."),
	), s.handleGetUser)

	s.mcpServer.AddTool(mcp.NewTool("get_summary",
		mcp.WithDescription("Get a summary of available lessons and reviews grouped by the hour."),
	), s.handleGetSummary)

	s.mcpServer.AddTool(mcp.NewTool("get_assignments",
		mcp.WithDescription(
			"Get SRS assignments with optional filters. "+
				"Each assignment tracks a user's progress on a specific subject through the SRS stages."),
		mcp.WithString("subjectTypes",
			mcp.Description("Comma-separated subject types to filter: radical, kanji, vocabulary, kana_vocabulary"),
		),
		mcp.WithString("levels",
			mcp.Description("Comma-separated WaniKani levels to filter (e.g. \"1,2,3\")"),
		),
		mcp.WithString("srsStages",
			mcp.Description("Comma-separated SRS stage numbers to filter (0=initiate through 9=burned)"),
		),
		mcp.WithString("availableBefore",
			mcp.Description("Only assignments available before this ISO 8601 datetime"),
		),
		mcp.WithString("availableAfter",
			mcp.Description("Only assignments available after this ISO 8601 datetime"),
		),
		mcp.WithBoolean("immediatelyAvailableForReview",
			mcp.Description("If true, only return assignments available for review right now"),
		),
		mcp.WithBoolean("immediatelyAvailableForLessons",
			mcp.Description("If true, only return assignments available for lessons right now"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of assignments to return (default: 500, max: 10000)"),
		),
	), s.handleGetAssignments)

	s.mcpServer.AddTool(mcp.NewTool("get_subjects",
		mcp.WithDescription(
			"Get WaniKani subjects (radicals, kanji, vocabulary). "+
				"Returns meanings, readings, mnemonics, and component relationships."),
		mcp.WithString("types",
			mcp.Description("Comma-separated subject types: radical, kanji, vocabulary, kana_vocabulary"),
		),
		mcp.WithString("levels",
			mcp.Description("Comma-separated WaniKani levels to filter (e.g. \"1,2,3\")"),
		),
		mcp.WithString("slugs",
			mcp.Description("Comma-separated slugs to look up specific subjects"),
		),
		mcp.WithString("ids",
			mcp.Description("Comma-separated subject IDs to fetch"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of subjects to return (default: 500, max: 10000)"),
		),
	), s.handleGetSubjects)

	s.mcpServer.AddTool(mcp.NewTool("get_review_statistics",
		mcp.WithDescription(
			"Get review accuracy statistics per subject, including correct/incorrect counts and streaks."),
		mcp.WithString("subjectTypes",
			mcp.Description("Comma-separated subject types: radical, kanji, vocabulary, kana_vocabulary"),
		),
		mcp.WithString("subjectIds",
			mcp.Description("Comma-separated subject IDs to filter"),
		),
		mcp.WithNumber("percentagesGreaterThan",
			mcp.Description("Only stats with percentage_correct greater than this value"),
		),
		mcp.WithNumber("percentagesLessThan",
			mcp.Description("Only stats with percentage_correct less than this value"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 500, max: 10000)"),
		),
	), s.handleGetReviewStatistics)

	s.mcpServer.AddTool(mcp.NewTool("get_level_progressions",
		mcp.WithDescription(
			"Get the user's progression history through WaniKani levels, "+
				"including timestamps for each stage."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 500, max: 10000)"),
		),
	), s.handleGetLevelProgressions)
}

func setOptionalString(params url.Values, args map[string]any, argKey, apiKey string) {
	if v, ok := args[argKey].(string); ok && v != "" {
		params.Set(apiKey, v)
	}
}

func setOptionalBool(params url.Values, args map[string]any, argKey, apiKey string) {
	if v, ok := args[argKey].(bool); ok {
		params.Set(apiKey, strconv.FormatBool(v))
	}
}

func setOptionalNumber(params url.Values, args map[string]any, argKey, apiKey string) {
	if v, ok := args[argKey].(float64); ok {
		params.Set(apiKey, strconv.Itoa(int(v)))
	}
}

func getLimit(args map[string]any) int {
	if v, ok := args["limit"].(float64); ok && v > 0 {
		return int(v)
	}
	return 0
}

func (s *Server) handleGetUser(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	user, err := s.client.GetUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get user: %v", err)), nil
	}
	return formatResource(user), nil
}

func (s *Server) handleGetSummary(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	summary, err := s.client.GetSummary(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get summary: %v", err)), nil
	}
	return formatResource(summary), nil
}

func (s *Server) handleGetAssignments(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "subjectTypes", "subject_types")
	setOptionalString(params, args, "levels", "levels")
	setOptionalString(params, args, "srsStages", "srs_stages")
	setOptionalString(params, args, "availableBefore", "available_before")
	setOptionalString(params, args, "availableAfter", "available_after")
	setOptionalBool(params, args, "immediatelyAvailableForReview", "immediately_available_for_review")
	setOptionalBool(params, args, "immediatelyAvailableForLessons", "immediately_available_for_lessons")

	items, total, err := s.client.GetAssignments(ctx, params, getLimit(args))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get assignments: %v", err)), nil
	}
	return formatCollection("assignment", items, total), nil
}

func (s *Server) handleGetSubjects(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "types", "types")
	setOptionalString(params, args, "levels", "levels")
	setOptionalString(params, args, "slugs", "slugs")

	if ids, ok := args["ids"].(string); ok && ids != "" {
		// Strip spaces: WaniKani requires bare comma-separated integers with no whitespace.
		params.Set("ids", strings.ReplaceAll(ids, " ", ""))
	}

	items, total, err := s.client.GetSubjects(ctx, params, getLimit(args))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get subjects: %v", err)), nil
	}
	return formatCollection("subject", items, total), nil
}

func (s *Server) handleGetReviewStatistics(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	params := url.Values{}

	setOptionalString(params, args, "subjectTypes", "subject_types")
	setOptionalString(params, args, "subjectIds", "subject_ids")
	setOptionalNumber(params, args, "percentagesGreaterThan", "percentages_greater_than")
	setOptionalNumber(params, args, "percentagesLessThan", "percentages_less_than")

	items, total, err := s.client.GetReviewStatistics(ctx, params, getLimit(args))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get review statistics: %v", err)), nil
	}
	return formatCollection("review statistic", items, total), nil
}

func (s *Server) handleGetLevelProgressions(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	items, total, err := s.client.GetLevelProgressions(ctx, getLimit(request.Params.Arguments))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get level progressions: %v", err)), nil
	}
	return formatCollection("level progression", items, total), nil
}

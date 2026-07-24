package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/service"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("list_server_emojis",
		mcp.WithDescription("List emoji in the configured Discord server for discovery and visual references."),
		mcp.WithString("query",
			mcp.Description("Optional case-insensitive name substring"),
		),
		mcp.WithBoolean("include_unavailable",
			mcp.Description("Include emoji that Discord reports as unavailable"),
		),
	), s.handleListServerEmojis)

	s.mcpServer.AddTool(mcp.NewTool("create_emoji",
		mcp.WithDescription(
			"Generate and upload a static Discord emoji, optionally matching existing server emoji. "+
				"Returns a mention that can be used immediately.",
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Emoji name: 2-32 letters, numbers, or underscores"),
		),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Description of the emoji to generate"),
		),
		mcp.WithArray("reference_emoji_ids",
			mcp.Description("Existing emoji IDs from list_server_emojis to use as visual references"),
			mcp.WithStringItems(),
		),
		mcp.WithArray("roles",
			mcp.Description("Optional Discord role IDs allowed to use the emoji"),
			mcp.WithStringItems(),
		),
	), s.handleCreateEmoji)

	s.mcpServer.AddTool(mcp.NewTool("list_created_emojis",
		mcp.WithDescription("List active emoji recorded as created by this MCP server."),
	), s.handleListCreatedEmojis)

	s.mcpServer.AddTool(mcp.NewTool("remove_emoji",
		mcp.WithDescription(
			"Remove an emoji only if Discord identifies the authenticated bot as its creator.",
		),
		mcp.WithString("emoji_id",
			mcp.Required(),
			mcp.Description("Immutable emoji ID returned by create_emoji or list_created_emojis"),
		),
	), s.handleRemoveEmoji)
}

func (s *Server) handleListServerEmojis(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, _ := request.GetArguments()["query"].(string)
	includeUnavailable, _ := request.GetArguments()["include_unavailable"].(bool)
	emojis, err := s.service.ListServerEmojis(ctx, query, includeUnavailable)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(emojis)
}

func (s *Server) handleCreateEmoji(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, _ := args["name"].(string)
	prompt, _ := args["prompt"].(string)
	emoji, err := s.service.CreateEmoji(ctx, service.CreateRequest{
		Name: name, Prompt: prompt,
		ReferenceIDs: stringSlice(args["reference_emoji_ids"]),
		Roles:        stringSlice(args["roles"]),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	data, err := json.MarshalIndent(emoji, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Created %s — ready to use immediately.\n\n%s", emoji.Mention, data),
	), nil
}

func (s *Server) handleListCreatedEmojis(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	emojis, err := s.service.ListCreatedEmojis(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(emojis)
}

func (s *Server) handleRemoveEmoji(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	emojiID, _ := request.GetArguments()["emoji_id"].(string)
	if err := s.service.RemoveEmoji(ctx, emojiID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("Removed emoji " + emojiID + "."), nil
}

func jsonResult(value any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	output := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			output = append(output, text)
		}
	}
	return output
}

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

	s.mcpServer.AddTool(mcp.NewTool("fetch_emoji_reference",
		mcp.WithDescription(
			"Fetch one static guild emoji by ID as base64 image data for an external image generator.",
		),
		mcp.WithString("emoji_id",
			mcp.Required(),
			mcp.Description("Stable emoji ID returned by list_server_emojis"),
		),
	), s.handleFetchEmojiReference)

	s.mcpServer.AddTool(mcp.NewTool("upload_emoji",
		mcp.WithDescription(
			"Prepare and upload host-generated image data as a static Discord emoji. "+
				"Accepts raw base64 or a PNG/JPEG/GIF data URL and returns an immediately usable mention.",
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Emoji name: 2-32 letters, numbers, or underscores"),
		),
		mcp.WithString("image_data",
			mcp.Required(),
			mcp.Description("Host-generated image as raw base64 or a base64 image data URL"),
		),
		mcp.WithArray("roles",
			mcp.Description("Optional Discord role IDs allowed to use the emoji"),
			mcp.WithStringItems(),
		),
	), s.handleUploadEmoji)

	s.mcpServer.AddTool(mcp.NewTool("list_created_emojis",
		mcp.WithDescription("List emoji Discord currently reports were created by this authenticated bot."),
	), s.handleListCreatedEmojis)

	s.mcpServer.AddTool(mcp.NewTool("remove_emoji",
		mcp.WithDescription(
			"Remove an emoji only if Discord identifies the authenticated bot as its creator.",
		),
		mcp.WithString("emoji_id",
			mcp.Required(),
			mcp.Description("Immutable emoji ID returned by upload_emoji or list_created_emojis"),
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

func (s *Server) handleFetchEmojiReference(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	emojiID, _ := request.GetArguments()["emoji_id"].(string)
	reference, err := s.service.FetchEmojiReference(ctx, emojiID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(reference)
}

func (s *Server) handleUploadEmoji(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, _ := args["name"].(string)
	imageData, _ := args["image_data"].(string)
	emoji, err := s.service.UploadEmoji(ctx, service.UploadRequest{
		Name: name, ImageData: imageData, Roles: stringSlice(args["roles"]),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	data, err := json.MarshalIndent(emoji, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Uploaded %s — ready to use immediately.\n\n%s", emoji.Mention, data),
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

package server

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// stringArrayItems is the JSON-schema item spec shared by every string-array tool argument.
var stringArrayItems = map[string]any{"type": "string"}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("list_asset_sources",
		mcp.WithDescription(
			"List the registered asset providers and, for each, the upstream sources it serves "+
				"(icon sets, illustration collections, font families) with license and item count. "+
				"Returns a human-readable listing plus a structured JSON block. Optionally filter by "+
				"kind (icon, illustration, font), by provider, or by source. Note: providers and "+
				"exclude_providers currently only match the offline embedded-* providers. No files are written."),
		mcp.WithString("kind",
			mcp.Description("Restrict to a single asset kind: icon, illustration, or font"),
		),
		mcp.WithArray("providers",
			mcp.Description("Only list these providers (e.g. embedded-icons)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("sources",
			mcp.Description("Only list these upstream sources (e.g. lucide, open-doodles, Inter)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these upstream sources"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithOutputSchema[providersManifest](),
	), s.handleListAssetSources)

	s.mcpServer.AddTool(mcp.NewTool("search_icons",
		mcp.WithDescription(
			"Search vendored icon sets (bootstrap-icons, feather, heroicons, lucide, material-symbols, "+
				"phosphor, simple-icons, tabler) by name. Returns a text list of hits, each with its "+
				"composite id (\"<provider>:<local>\", e.g. embedded-icons:lucide/camera) and a set/name "+
				"label. No files are written; pass a hit's id to get_icon to render it."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against icon names"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these icon sets (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these icon sets"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these icon providers (currently only embedded-icons)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these icon providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchIcons)

	s.mcpServer.AddTool(mcp.NewTool("get_icon",
		mcp.WithDescription(
			"Render a single icon to a standalone SVG file and write it to disk. The id is the composite "+
				"identifier from search_icons, formatted \"<provider>:<local>\" (e.g. "+
				"embedded-icons:lucide/camera), which you may also construct directly. "+
				"For the simple-icons set: the vector data is CC0-1.0, but the brand marks it depicts are "+
				"third-party trademarks — using them must not imply endorsement."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite icon id from search_icons, e.g. embedded-icons:lucide/camera"),
		),
		mcp.WithString("color",
			mcp.Description("CSS color to set on the SVG root, applied via a color attribute that "+
				"currentColor fills/strokes resolve to; omit for none"),
		),
		mcp.WithNumber("size",
			mcp.Description("Output width/height in pixels; omit to use the icon's native grid size"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetIcon)

	s.mcpServer.AddTool(mcp.NewTool("search_illustrations",
		mcp.WithDescription(
			"Search vendored SVG illustration collections (open-doodles, humaaans, open-peeps) by name. "+
				"Returns a text list of hits, each with its composite id "+
				"(\"<provider>:<local>\", e.g. embedded-illustrations:open-doodles/coffee-doodle) and a "+
				"collection/name label. No files are written; pass a hit's id to get_illustration."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against illustration names"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these collections (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these collections"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these illustration providers (currently only embedded-illustrations)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these illustration providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchIllustrations)

	s.mcpServer.AddTool(mcp.NewTool("get_illustration",
		mcp.WithDescription(
			"Fetch a single SVG illustration and write it to disk, unmodified. The id is the composite "+
				"identifier from search_illustrations, formatted \"<provider>:<local>\" (e.g. "+
				"embedded-illustrations:open-doodles/coffee-doodle), which you may also construct directly."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite illustration id from search_illustrations, e.g. "+
				"embedded-illustrations:open-doodles/coffee-doodle"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetIllustration)

	s.mcpServer.AddTool(mcp.NewTool("search_fonts",
		mcp.WithDescription(
			"Search vendored OFL-1.1 font families by name, slug, or category. Returns a text list of "+
				"hits, each with its composite id (\"<provider>:<local>\", e.g. embedded-fonts:inter), the "+
				"family category, and available weights. No files are written; pass a hit's id to get_font."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against family name, slug, or category"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these font families by display name (see list_asset_sources)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these font families by display name"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these font providers (currently only embedded-fonts)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these font providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchFonts)

	s.mcpServer.AddTool(mcp.NewTool("get_font",
		mcp.WithDescription(
			"Fetch a font family's woff2 file and write it to disk. The id is the composite identifier "+
				"from search_fonts, formatted \"<provider>:<local>\" (e.g. embedded-fonts:inter), which you "+
				"may also construct directly. With format=\"css\", also writes an @font-face CSS snippet "+
				"referencing the woff2 file."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite font id from search_fonts, e.g. embedded-fonts:inter"),
		),
		mcp.WithNumber("weight",
			mcp.Description("Font weight: 400 or 700 (default: 400; Bebas Neue only has 400)"),
		),
		mcp.WithString("style",
			mcp.Description("Font style; only \"normal\" is available (default: normal)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: \"woff2\" (default) or \"css\" to also emit an @font-face snippet"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetFont)
}

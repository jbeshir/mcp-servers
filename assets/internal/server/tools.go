package server

import (
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("list_asset_sources",
		mcp.WithDescription(
			"List every vendored asset source (icon sets, illustration collections, font families) "+
				"with its license, attribution, and item count. Returns a human-readable catalogue plus "+
				"the full catalog as a JSON text block. No files are written."),
	), s.handleListAssetSources)

	s.mcpServer.AddTool(mcp.NewTool("search_icons",
		mcp.WithDescription(
			"Search vendored icon sets (bootstrap-icons, feather, heroicons, lucide, material-symbols, "+
				"phosphor, simple-icons, tabler) by name. Returns a text list of matching set/name pairs. "+
				"No files are written; use get_icon to render one."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against icon names"),
		),
		mcp.WithString("set",
			mcp.Description("Restrict the search to a single icon set (see list_asset_sources for names)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
	), s.handleSearchIcons)

	s.mcpServer.AddTool(mcp.NewTool("get_icon",
		mcp.WithDescription(
			"Render a single icon to a standalone SVG file and write it to disk. "+
				"For the simple-icons set: the vector data is CC0-1.0, but the brand marks it depicts are "+
				"third-party trademarks — using them must not imply endorsement."),
		mcp.WithString("set",
			mcp.Required(),
			mcp.Description("Icon set name (see list_asset_sources)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Icon name within the set (see search_icons)"),
		),
		mcp.WithString("color",
			mcp.Description("CSS color to set on the SVG root, applied via a color attribute that "+
				"currentColor fills/strokes resolve to; omit for none"),
		),
		mcp.WithNumber("size",
			mcp.Description("Output width/height in pixels; omit to use the icon's native grid size"),
		),
	), s.handleGetIcon)

	s.mcpServer.AddTool(mcp.NewTool("search_illustrations",
		mcp.WithDescription(
			"Search vendored SVG illustration collections (open-doodles, humaaans, open-peeps) by name. "+
				"Returns a text list of matching collection/name pairs. "+
				"No files are written; use get_illustration to fetch one."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against illustration names"),
		),
		mcp.WithString("collection",
			mcp.Description("Restrict the search to a single collection (see list_asset_sources for names)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
	), s.handleSearchIllustrations)

	s.mcpServer.AddTool(mcp.NewTool("get_illustration",
		mcp.WithDescription("Fetch a single SVG illustration and write it to disk, unmodified."),
		mcp.WithString("collection",
			mcp.Required(),
			mcp.Description("Illustration collection name (see list_asset_sources)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Illustration name within the collection (see search_illustrations)"),
		),
	), s.handleGetIllustration)

	s.mcpServer.AddTool(mcp.NewTool("search_fonts",
		mcp.WithDescription(
			"Search vendored OFL-1.1 font families by name, slug, or category. "+
				"Returns a text list of families with their category and available weights. "+
				"No files are written; use get_font to fetch a variant."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive substring to match against family name, slug, or category"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
	), s.handleSearchFonts)

	s.mcpServer.AddTool(mcp.NewTool("get_font",
		mcp.WithDescription(
			"Fetch a font family's woff2 file and write it to disk. "+
				"With format=\"css\", also writes an @font-face CSS snippet referencing the woff2 file."),
		mcp.WithString("family",
			mcp.Required(),
			mcp.Description("Font family display name or slug (see list_asset_sources / search_fonts)"),
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
	), s.handleGetFont)
}

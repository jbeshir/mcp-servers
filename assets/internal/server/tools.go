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
				"(icon sets, illustration collections, font families, photo sources, texture/material "+
				"sets, 3D model sources, audio sources, sprite packs) with license and item count. "+
				"Returns a human-readable listing "+
				"plus a structured JSON block. Optionally filter by kind (icon, illustration, font, "+
				"photo, texture, model, audio, sprite), by provider, or by source."),
		mcp.WithString("kind",
			mcp.Description(
				"Restrict to a single asset kind: icon, illustration, font, photo, texture, model, audio, or sprite",
			),
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
			"Lexical name search, not vector or semantic search: use short literal terms, not natural-language prose. "+
				"Search vendored icon sets (bootstrap-icons, feather, heroicons, lucide, material-symbols, "+
				"phosphor, simple-icons, tabler) plus Iconify's remote catalogue by name. Returns a text "+
				"list of hits, each with its composite id (\"<provider>:<local>\", e.g. "+
				"embedded-icons:lucide/camera) and a set/name label. No files are written; pass a hit's "+
				"id to get_icon to render it."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description(
				"Short literal icon name term; local sets use case-insensitive substring matching, "+
					"while remote providers apply their own lexical matching"),
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
			mcp.Description("Restrict to these icon providers (embedded-icons, iconify)"),
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
			"Case-insensitive lexical name search, not vector or semantic search: use a short literal term, "+
				"not natural-language prose. "+
				"Search vendored SVG illustration collections (open-doodles, humaaans, open-peeps) by name. "+
				"Returns a text list of hits, each with its composite id "+
				"(\"<provider>:<local>\", e.g. embedded-illustrations:open-doodles/coffee-doodle) and a "+
				"collection/name label. No files are written; pass a hit's id to get_illustration."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Short literal term matched case-insensitively as a substring of illustration names"),
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
			"Lexical name/tag/category search, not vector or semantic search: use short literal terms, "+
				"not natural-language prose. "+
				"Search vendored OFL-1.1 font families plus the Google Fonts catalogue by name, slug, or "+
				"category. Returns a text list of hits, each with its composite id "+
				"(\"<provider>:<local>\", e.g. embedded-fonts:inter), the family category, and available "+
				"weights. No files are written; pass a hit's id to get_font. Game-art results include their "+
				"pack; search all needs first and prefer get_pack when several share one."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description(
				"Short literal family name, slug, tag, or category term; local catalogs use case-insensitive "+
					"substring matching, while remote providers apply their own lexical matching"),
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
			mcp.Description("Restrict to these font providers (embedded-fonts, googlefonts, assetsdb)"),
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
			mcp.Description("Font weight, e.g. 400 or 700 (default: 400); embedded families offer "+
				"400/700, Google Fonts families vary"),
		),
		mcp.WithString("style",
			mcp.Description("Font style: \"normal\" or \"italic\" (default: normal); availability depends on the family"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: \"woff2\" (default) or \"css\" to also emit an @font-face snippet"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetFont)

	s.mcpServer.AddTool(mcp.NewTool("search_photos",
		mcp.WithDescription(
			"Lexical provider search, not vector or semantic search: use short literal name or tag terms, "+
				"not natural-language prose. "+
				"Search keyless Openverse CC-licensed photos by name. Returns a text list of hits, each "+
				"with its composite id (\"<provider>:<local>\") and a source/title label. No files are "+
				"written; pass a hit's id to get_photo to fetch it."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Short literal photo name or tag term interpreted lexically by Openverse"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these upstream sources (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these upstream sources"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these photo providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these photo providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchPhotos)

	s.mcpServer.AddTool(mcp.NewTool("get_photo",
		mcp.WithDescription(
			"Fetch a photo by composite id and write it to disk. The id is the composite identifier "+
				"from search_photos, formatted \"<provider>:<local>\"."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite photo id from search_photos"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetPhoto)

	s.mcpServer.AddTool(mcp.NewTool("search_textures",
		mcp.WithDescription(
			"Lexical provider search, not vector or semantic search: use short literal material/category terms, "+
				"not natural-language prose. "+
				"Search keyless CC0 ambientCG PBR material sets by name. Returns a text list of hits, "+
				"each with its composite id (\"<provider>:<local>\") and a source/title label. No files "+
				"are written; pass a hit's id to get_texture to fetch it."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Short literal material name or category term interpreted lexically by ambientCG"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these upstream sources (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these upstream sources"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these texture providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these texture providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchTextures)

	s.mcpServer.AddTool(mcp.NewTool("get_texture",
		mcp.WithDescription(
			"Fetch a PBR material archive (a zip of texture maps) and write it to disk. The id is the "+
				"composite identifier from search_textures, formatted \"<provider>:<local>\"."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite texture id from search_textures"),
		),
		mcp.WithString("resolution",
			mcp.Description("Texture resolution, e.g. 1K, 2K, 4K (default: 1K)"),
		),
		mcp.WithString("format",
			mcp.Description("Texture map format, e.g. JPG, PNG (default: JPG)"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetTexture)

	s.mcpServer.AddTool(mcp.NewTool("search_models",
		mcp.WithDescription(
			"Lexical name/tag search, not vector or semantic search: use short literal terms, not natural-language prose. "+
				"Search 3D model providers (assetsdb, Poly Pizza, Poly Haven) by name. Returns a text list of hits, "+
				"each with its composite id (\"<provider>:<local>\") and a source/title label. Results "+
				"come from local and opt-in providers, so an empty result may mean no provider is "+
				"configured. No files are written; pass a hit's id to get_model to fetch it. Game-art results "+
				"include their pack; search all needs first and prefer get_pack when several share one."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description(
				"Short literal model name or tag term; AssetsDB uses case-insensitive name/token substring "+
					"matching, while remote providers apply their own lexical matching"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these upstream sources (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these upstream sources"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these model providers (assetsdb, polypizza, polyhaven)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these model providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchModels)

	s.mcpServer.AddTool(mcp.NewTool("get_model",
		mcp.WithDescription(
			"Fetch a 3D model (a glTF/GLB file, or a zip of a glTF plus its referenced assets) and "+
				"write it to disk. The id is the composite identifier from search_models, formatted "+
				"\"<provider>:<local>\"."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite model id from search_models"),
		),
		mcp.WithString("format",
			mcp.Description("Model file format, e.g. glb, gltf (default: provider default)"),
		),
		mcp.WithString("resolution",
			mcp.Description("Texture resolution for models with multi-resolution PBR textures "+
				"(default: provider default)"),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetModel)

	s.mcpServer.AddTool(mcp.NewTool("search_audio",
		mcp.WithDescription(
			"Lexical name/tag search, not vector or semantic search: use short literal terms, not natural-language prose. "+
				"Search audio providers (assetsdb, Jamendo, Freesound) by name. Returns a text list of hits, each "+
				"with its composite id (\"<provider>:<local>\") and a source/title label. Results come "+
				"from local and opt-in providers, so an empty result may mean no provider is "+
				"configured. No files are written; pass a hit's id to get_audio to fetch it. Game-art results "+
				"include their pack; search all needs first and prefer get_pack when several share one."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description(
				"Short literal audio name or tag term; AssetsDB uses case-insensitive name/token substring "+
					"matching, while remote providers apply their own lexical matching"),
		),
		mcp.WithArray("sources",
			mcp.Description("Restrict to these upstream sources (see list_asset_sources for names)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_sources",
			mcp.Description("Omit these upstream sources"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("providers",
			mcp.Description("Restrict to these audio providers (assetsdb, jamendo, freesound)"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithArray("exclude_providers",
			mcp.Description("Omit these audio providers"),
			mcp.Items(stringArrayItems),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous search's next_cursor; omit for the first page"),
		),
	), s.handleSearchAudio)

	s.mcpServer.AddTool(mcp.NewTool("get_audio",
		mcp.WithDescription(
			"Fetch an audio clip (mp3 or ogg) and write it to disk. The id is the composite identifier "+
				"from search_audio, formatted \"<provider>:<local>\". Note: for Freesound, the fetched "+
				"audio is its high-quality preview, not the original master file."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Composite audio id from search_audio"),
		),
		mcp.WithString("format",
			mcp.Description("Audio encoding: \"mp3\" (default) or \"ogg\""),
		),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetAudio)

	s.mcpServer.AddTool(mcp.NewTool("search_sprites",
		mcp.WithDescription(
			"Case-insensitive lexical name/token substring search, not vector or semantic search: use short literal "+
				"sprite names, tags, or categories, not natural-language prose. Search local assetsdb game-art "+
				"sprites. Results include pack metadata. Search for everything "+
				"you need first; when several results share a pack, prefer one get_pack over many individual fetches."),
		mcp.WithString("query", mcp.Required(),
			mcp.Description(
				"Short literal sprite name, tag, or category term matched case-insensitively against "+
					"AssetsDB names and tokens")),
		mcp.WithArray("sources", mcp.Items(stringArrayItems)),
		mcp.WithArray("exclude_sources", mcp.Items(stringArrayItems)),
		mcp.WithArray("providers", mcp.Items(stringArrayItems)),
		mcp.WithArray("exclude_providers", mcp.Items(stringArrayItems)),
		mcp.WithNumber("limit"),
		mcp.WithString("cursor"),
	), s.handleSearchSprites)
	s.mcpServer.AddTool(mcp.NewTool("get_sprite",
		mcp.WithDescription(
			"Write a sprite returned by search_sprites; atlas results write the complete sheet and expose region metadata."),
		mcp.WithString("id", mcp.Required()),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetSprite)
	s.mcpServer.AddTool(mcp.NewTool("list_pack_assets",
		mcp.WithDescription(
			"List the catalogued contents of one AssetsDB pack without a search query. Use this to inspect a known "+
				"pack from list_asset_sources. Results use the same asset lines as search, including pack and "+
				"atlas-region metadata."),
		mcp.WithString("pack_id", mcp.Required(),
			mcp.Description("AssetsDB pack ID from list_asset_sources")),
		mcp.WithString("kind",
			mcp.Description("Optional asset kind filter: model, audio, font, or sprite")),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)")),
		mcp.WithString("cursor",
			mcp.Description("Opaque pagination token from a previous list's next_cursor; omit for the first page")),
	), s.handleListPackAssets)
	s.mcpServer.AddTool(mcp.NewTool("get_pack",
		mcp.WithDescription(
			"Copy a known assetsdb pack's original ZIP. Search all needs first; when several results share a "+
				"pack, prefer this tool over individual fetches."),
		mcp.WithString("pack_id", mcp.Required(),
			mcp.Description("Known pack ID from list_asset_sources")),
		mcp.WithOutputSchema[fileManifest](),
	), s.handleGetPack)
}

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultFontWeight        = 400
	defaultFontStyle         = "normal"
	cssFormat                = "css"
	simpleIconsSet           = "simple-icons"
	defaultTextureResolution = "1K"
	defaultTextureFormat     = "JPG"
	defaultAudioFormat       = "mp3"
)

// stringArg reads a string argument, defaulting to "" if absent or of the wrong type.
func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// intArg reads a numeric argument (mcp-go delivers JSON numbers as float64), defaulting to def if
// absent or of the wrong type.
func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}

	return def
}

// stringSliceArg reads a string-array argument (JSON arrays arrive as []any), dropping non-string and
// empty elements. Absent or wrong-typed arguments yield nil.
func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}

	return out
}

// filterArg builds an assetcore.Filter from an include ("only") and exclude argument pair.
func filterArg(args map[string]any, onlyKey, exceptKey string) assetcore.Filter {
	return assetcore.Filter{
		Only:   stringSliceArg(args, onlyKey),
		Except: stringSliceArg(args, exceptKey),
	}
}

// sanitizeFilename replaces every character outside [a-zA-Z0-9_-] with a hyphen.
func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, s)
}

// sanitizeSuffixedFilename sanitizes prefix+name's stem while preserving name's extension. It is for the
// kinds (photo, texture, model) whose Blob.Filename already carries a provider-supplied extension,
// unlike icon/illustration/font where the extension is hardcoded and appended after sanitizing the stem.
func sanitizeSuffixedFilename(prefix, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	return sanitizeFilename(prefix+stem) + ext
}

// sourceTitleLines renders one "id — source/title" line per asset.
func sourceTitleLines(assets []assetcore.Asset) []string {
	lines := make([]string, 0, len(assets))
	for _, a := range assets {
		line := fmt.Sprintf("%s — %s/%s", a.ID, a.Source, a.Title)
		if pack := a.Meta[assetcore.MetaPack]; pack != "" {
			line += fmt.Sprintf(" [pack=%s pack_title=%q]", pack, a.Meta[assetcore.MetaPackTitle])
		}
		if region := regionText(a.Meta); region != "" {
			line += " [region=" + region + "]"
		}
		lines = append(lines, line)
	}
	return lines
}

func regionText(m map[string]string) string {
	if m["region_width"] == "" {
		return ""
	}
	return strings.Join([]string{m["region_x"], m["region_y"], m["region_width"], m["region_height"]}, ",")
}

// searchResult renders a search header plus one line per hit (or a "no matches" note), a next_cursor
// line when nextCursor is non-empty, and a note naming any providers that degraded during the
// aggregate search.
func searchResult(header string, lines []string, nextCursor string, warns []assetcore.Warning) *mcp.CallToolResult {
	var b strings.Builder
	b.WriteString(header)

	if len(lines) == 0 {
		b.WriteString("\n(no matches)")
	} else {
		b.WriteString("\n")
		b.WriteString(strings.Join(lines, "\n"))
	}

	if nextCursor != "" {
		fmt.Fprintf(&b, "\nnext_cursor: %s", nextCursor)
	}

	if len(warns) > 0 {
		names := make([]string, 0, len(warns))
		for _, w := range warns {
			names = append(names, w.Provider)
		}
		fmt.Fprintf(&b, "\nNote: some providers were unavailable and omitted: %s", strings.Join(names, ", "))
	}

	return mcp.NewToolResultText(b.String())
}

// sourceJSON is the structured shape of one upstream source in list_asset_sources output.
type sourceJSON struct {
	Name    string            `json:"name"`
	License string            `json:"license"`
	Count   int               `json:"count"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// providerJSON is the structured shape of one provider in list_asset_sources output.
type providerJSON struct {
	Provider string       `json:"provider"`
	Kind     string       `json:"kind"`
	Sources  []sourceJSON `json:"sources"`
}

// providersManifest is the JSON shape emitted as native structured content by list_asset_sources.
type providersManifest struct {
	Providers []providerJSON `json:"providers"`
	Packs     []packJSON     `json:"packs,omitempty"`
}

type packJSON struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Tags    []string `json:"tags,omitempty"`
	Count   int      `json:"count"`
	License string   `json:"license"`
}

func (s *Server) handleListAssetSources(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	kindFilter := stringArg(args, "kind")
	providers := filterArg(args, "providers", "exclude_providers")
	sources := filterArg(args, "sources", "exclude_sources")
	sourceScoped := len(sources.Only) > 0 || len(sources.Except) > 0

	var (
		b   strings.Builder
		out []providerJSON
	)

	for _, info := range s.registry.Providers() {
		if !includeProvider(info, kindFilter, providers) {
			continue
		}

		srcs := filterSources(info.Sources, sources)

		// When a source filter is active, a provider whose sources are all filtered out is noise.
		if sourceScoped && len(srcs) == 0 {
			continue
		}

		out = append(out, providerJSON{Provider: info.Name, Kind: string(info.Kind), Sources: srcs})
		writeProviderListing(&b, info, srcs)
	}

	if len(out) == 0 {
		b.WriteString("(no matching sources)\n")
	}

	packOut := s.filteredPacks(kindFilter, providers, sources)
	if len(packOut) > 0 {
		b.WriteString("assetsdb packs:\n")
		for _, p := range packOut {
			fmt.Fprintf(&b, "  %s — %s — %s — %d\n", p.ID, p.Title, p.License, p.Count)
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(b.String()),
		},
		StructuredContent: providersManifest{Providers: out, Packs: packOut},
	}, nil
}

func (s *Server) filteredPacks(kind string, providers, sources assetcore.Filter) []packJSON {
	if !providers.Allows("assetsdb") {
		return nil
	}
	out := []packJSON{}
	for _, p := range s.packStore.Packs() {
		if !sources.Allows(p.ID) {
			continue
		}
		if kind != "" && p.Kinds[assetcore.Kind(kind)] == 0 {
			continue
		}
		out = append(out, packJSON{ID: p.ID, Title: p.Title, Tags: p.Tags, Count: p.Count, License: p.License.SPDX})
	}
	return out
}

// includeProvider reports whether a provider passes the kind and provider filters.
func includeProvider(info assetcore.ProviderInfo, kind string, providers assetcore.Filter) bool {
	if kind != "" && string(info.Kind) != kind {
		return false
	}

	return providers.Allows(info.Name)
}

// filterSources projects the sources a provider serves through the source filter into the JSON shape.
func filterSources(sources []assetcore.Source, f assetcore.Filter) []sourceJSON {
	out := make([]sourceJSON, 0, len(sources))
	for _, src := range sources {
		if !f.Allows(src.Name) {
			continue
		}
		out = append(out, sourceJSON{
			Name:    src.Name,
			License: src.License.SPDX,
			Count:   src.Count,
			Meta:    src.Meta,
		})
	}

	return out
}

// writeProviderListing renders a provider header and its (already filtered) source lines.
func writeProviderListing(b *strings.Builder, info assetcore.ProviderInfo, srcs []sourceJSON) {
	fmt.Fprintf(b, "%s (%s):\n", info.Name, info.Kind)
	if len(srcs) == 0 {
		b.WriteString("  (no enumerable sources)\n")
		return
	}
	for _, src := range srcs {
		writeSourceLine(b, src)
	}
}

// writeSourceLine renders one source's listing line, appending its count and any category Meta.
func writeSourceLine(b *strings.Builder, src sourceJSON) {
	b.WriteString("  ")
	b.WriteString(src.Name)
	if src.License != "" {
		b.WriteString(" — ")
		b.WriteString(src.License)
	}
	if src.Count >= 0 {
		fmt.Fprintf(b, " — %d", src.Count)
	}
	if cat := src.Meta[assetcore.MetaCategory]; cat != "" {
		fmt.Fprintf(b, " [%s]", cat)
	}
	b.WriteString("\n")
}

func (s *Server) handleSearchIcons(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchIcons(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(fmt.Sprintf("%d icon(s) matching %q:", len(assets), query), lines, nextCursor, warns), nil
}

func (s *Server) handleGetIcon(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	color := stringArg(args, "color")
	size := intArg(args, "size", 0)

	blob, err := s.registry.FetchIcon(ctx, id, assetcore.IconFetchOpts{Color: color, Size: size})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError("icon not found: " + id), nil
		}

		return nil, fmt.Errorf("render icon %s: %w", id, err)
	}

	set, name := blob.Asset.Source, blob.Asset.Title

	filename := sanitizeFilename(fmt.Sprintf("icon-%s-%s", set, name))
	if size > 0 {
		filename += fmt.Sprintf("-%d", size)
	}
	if color != "" {
		filename += "-" + sanitizeFilename(color)
	}
	filename += ".svg"

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write icon %s: %w", id, err)
	}

	summary := fmt.Sprintf("Wrote icon %s to %s", id, path)
	if strings.EqualFold(set, simpleIconsSet) {
		summary += ". Note: simple-icons vector data is CC0-1.0, but the brand mark it depicts is a " +
			"third-party trademark — using it must not imply endorsement."
	}

	return newFileResult(summary, []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchIllustrations(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchIllustrations(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(
		fmt.Sprintf("%d illustration(s) matching %q:", len(assets), query), lines, nextCursor, warns,
	), nil
}

func (s *Server) handleGetIllustration(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	blob, err := s.registry.FetchIllustration(ctx, id)
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError("illustration not found: " + id), nil
		}

		return nil, fmt.Errorf("get illustration %s: %w", id, err)
	}

	collection, name := blob.Asset.Source, blob.Asset.Title

	filename := sanitizeFilename(fmt.Sprintf("illustration-%s-%s", collection, name)) + ".svg"

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write illustration %s: %w", id, err)
	}

	return newFileResult(fmt.Sprintf("Wrote illustration %s to %s", id, path), []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchFonts(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchFonts(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := make([]string, 0, len(assets))
	for _, a := range assets {
		lines = append(lines, fmt.Sprintf(
			"%s — %s (%s) weights: %s",
			a.ID, a.Title, a.Meta[assetcore.MetaCategory], a.Meta[assetcore.MetaWeights],
		))
	}

	return searchResult(
		fmt.Sprintf("%d font family(-ies) matching %q:", len(assets), query), lines, nextCursor, warns,
	), nil
}

func (s *Server) handleGetFont(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	weight := intArg(args, "weight", defaultFontWeight)

	style := stringArg(args, "style")
	if style == "" {
		style = defaultFontStyle
	}

	format := stringArg(args, "format")

	blob, prov, err := s.registry.FetchFont(ctx, id, assetcore.FontFetchOpts{Weight: weight, Style: style})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("font not found: %s weight=%d style=%s", id, weight, style)), nil
		}

		return nil, fmt.Errorf("get font %s: %w", id, err)
	}

	family := blob.Asset.Title

	slug := sanitizeFilename(strings.ToLower(family))
	base := fmt.Sprintf("font-%s-%d-%s", slug, weight, style)

	woffPath, err := writeAsset(s.outputDir, base+".woff2", blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write font %s: %w", id, err)
	}

	files := []manifestFile{manifestFileFor(woffPath, blob)}

	summary := fmt.Sprintf("Wrote font %s (weight %d, %s) to %s", family, weight, style, woffPath)

	if strings.EqualFold(format, cssFormat) {
		renderer, ok := prov.(assetcore.FontFaceRenderer)
		if !ok {
			return nil, fmt.Errorf("font provider %q cannot render @font-face CSS", prov.Name())
		}

		css := renderer.RenderFontFace(family, blob)

		cssPath, err := writeAsset(s.outputDir, base+".css", []byte(css))
		if err != nil {
			return nil, fmt.Errorf("write font css %s: %w", id, err)
		}

		cssEntry := manifestFileFor(cssPath, blob)
		cssEntry.ContentType = "text/css"
		files = append(files, cssEntry)
		summary += fmt.Sprintf(" and @font-face CSS to %s", cssPath)
	}

	return newFileResult(summary, files)
}

func (s *Server) handleSearchPhotos(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchPhotos(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(fmt.Sprintf("%d photo(s) matching %q:", len(assets), query), lines, nextCursor, warns), nil
}

func (s *Server) handleGetPhoto(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	blob, err := s.registry.FetchPhoto(ctx, id, assetcore.PhotoFetchOpts{})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError("photo not found: " + id), nil
		}

		return nil, fmt.Errorf("get photo %s: %w", id, err)
	}

	filename := sanitizeSuffixedFilename("photo-", blob.Filename)

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write photo %s: %w", id, err)
	}

	return newFileResult(fmt.Sprintf("Wrote photo %s to %s", id, path), []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchTextures(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchTextures(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(fmt.Sprintf("%d texture(s) matching %q:", len(assets), query), lines, nextCursor, warns), nil
}

func (s *Server) handleGetTexture(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	resolution := stringArg(args, "resolution")
	if resolution == "" {
		resolution = defaultTextureResolution
	}

	format := stringArg(args, "format")
	if format == "" {
		format = defaultTextureFormat
	}

	blob, err := s.registry.FetchTexture(ctx, id, assetcore.TextureFetchOpts{Resolution: resolution, Format: format})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError(
				fmt.Sprintf("texture not found: %s resolution=%s format=%s", id, resolution, format),
			), nil
		}

		return nil, fmt.Errorf("get texture %s: %w", id, err)
	}

	filename := sanitizeSuffixedFilename("", blob.Filename)

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write texture %s: %w", id, err)
	}

	return newFileResult(fmt.Sprintf("Wrote texture %s to %s", id, path), []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchModels(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchModels(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(fmt.Sprintf("%d model(s) matching %q:", len(assets), query), lines, nextCursor, warns), nil
}

func (s *Server) handleGetModel(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	format := stringArg(args, "format")
	resolution := stringArg(args, "resolution")

	blob, err := s.registry.FetchModel(ctx, id, assetcore.ModelFetchOpts{Format: format, Resolution: resolution})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError(
				fmt.Sprintf("model not found: %s format=%s resolution=%s", id, format, resolution),
			), nil
		}

		return nil, fmt.Errorf("get model %s: %w", id, err)
	}

	filename := sanitizeSuffixedFilename("model-", blob.Filename)

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write model %s: %w", id, err)
	}

	return newFileResult(fmt.Sprintf("Wrote model %s to %s", id, path), []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchAudio(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	assets, nextCursor, warns := s.registry.SearchAudio(ctx, assetcore.SearchOpts{
		Query:     query,
		Cursor:    stringArg(args, "cursor"),
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := sourceTitleLines(assets)

	return searchResult(fmt.Sprintf("%d audio clip(s) matching %q:", len(assets), query), lines, nextCursor, warns), nil
}

func (s *Server) handleGetAudio(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id := stringArg(args, "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	format := stringArg(args, "format")
	if format == "" {
		format = defaultAudioFormat
	}

	blob, err := s.registry.FetchAudio(ctx, id, assetcore.AudioFetchOpts{Format: format})
	if err != nil {
		if errors.Is(err, assetcore.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("audio not found: %s format=%s", id, format)), nil
		}

		return nil, fmt.Errorf("get audio %s: %w", id, err)
	}

	filename := sanitizeSuffixedFilename("audio-", blob.Filename)

	path, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write audio %s: %w", id, err)
	}

	return newFileResult(fmt.Sprintf("Wrote audio %s to %s", id, path), []manifestFile{manifestFileFor(path, blob)})
}

func (s *Server) handleSearchSprites(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	assets, next, warns := s.registry.SearchSprites(ctx, assetcore.SearchOpts{
		Query: query, Cursor: stringArg(args, "cursor"), Limit: intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})
	message := fmt.Sprintf("%d sprite(s) matching %q:", len(assets), query)
	return searchResult(message, sourceTitleLines(assets), next, warns), nil
}

func (s *Server) handleGetSprite(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := stringArg(request.GetArguments(), "id")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}
	blob, err := s.registry.FetchSprite(ctx, id, assetcore.SpriteFetchOpts{})
	if errors.Is(err, assetcore.ErrNotFound) {
		return mcp.NewToolResultError("sprite not found: " + id), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sprite %s: %w", id, err)
	}
	filename := sanitizeSuffixedFilename("sprite-", blob.Filename)
	out, err := writeAsset(s.outputDir, filename, blob.Content)
	if err != nil {
		return nil, fmt.Errorf("write sprite %s: %w", id, err)
	}
	return newFileResult(fmt.Sprintf("Wrote sprite %s to %s", id, out), []manifestFile{manifestFileFor(out, blob)})
}

func (s *Server) handleGetPack(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := stringArg(request.GetArguments(), "pack_id")
	if id == "" {
		return mcp.NewToolResultError("pack_id is required"), nil
	}
	r, p, err := s.packStore.OpenPack(id)
	if errors.Is(err, assetcore.ErrNotFound) {
		return mcp.NewToolResultError("pack not found: " + id), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open pack %s: %w", id, err)
	}
	data, readErr := io.ReadAll(r)
	closeErr := r.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read pack %s: %w", id, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close pack %s: %w", id, closeErr)
	}
	blob := assetcore.Blob{
		Asset: assetcore.Asset{
			ID: "assetsdb:" + id, Kind: assetcore.Kind("pack"), Source: id, Title: p.Title, License: p.License,
		},
		Content: data, Filename: sanitizeFilename(id) + ".zip", ContentType: "application/zip",
	}
	out, err := writeAsset(s.outputDir, blob.Filename, data)
	if err != nil {
		return nil, fmt.Errorf("write pack %s: %w", id, err)
	}
	return newFileResult(fmt.Sprintf("Wrote assetsdb pack %s to %s", id, out), []manifestFile{manifestFileFor(out, blob)})
}

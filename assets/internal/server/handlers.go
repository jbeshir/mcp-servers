package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultFontWeight = 400
	defaultFontStyle  = "normal"
	cssFormat         = "css"
	simpleIconsSet    = "simple-icons"
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

// searchResult renders a search header plus one line per hit (or a "no matches" note) and appends a
// note naming any providers that degraded during the aggregate search.
func searchResult(header string, lines []string, warns []assetcore.Warning) *mcp.CallToolResult {
	var b strings.Builder
	b.WriteString(header)

	if len(lines) == 0 {
		b.WriteString("\n(no matches)")
	} else {
		b.WriteString("\n")
		b.WriteString(strings.Join(lines, "\n"))
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

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(b.String()),
		},
		StructuredContent: providersManifest{Providers: out},
	}, nil
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

	page, warns := s.registry.SearchIcons(ctx, assetcore.SearchOpts{
		Query:     query,
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := make([]string, 0, len(page.Assets))
	for _, a := range page.Assets {
		lines = append(lines, fmt.Sprintf("%s — %s/%s", a.ID, a.Source, a.Title))
	}

	return searchResult(fmt.Sprintf("%d icon(s) matching %q:", len(page.Assets), query), lines, warns), nil
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

	page, warns := s.registry.SearchIllustrations(ctx, assetcore.SearchOpts{
		Query:     query,
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := make([]string, 0, len(page.Assets))
	for _, a := range page.Assets {
		lines = append(lines, fmt.Sprintf("%s — %s/%s", a.ID, a.Source, a.Title))
	}

	return searchResult(fmt.Sprintf("%d illustration(s) matching %q:", len(page.Assets), query), lines, warns), nil
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

	page, warns := s.registry.SearchFonts(ctx, assetcore.SearchOpts{
		Query:     query,
		Limit:     intArg(args, "limit", 0),
		Sources:   filterArg(args, "sources", "exclude_sources"),
		Providers: filterArg(args, "providers", "exclude_providers"),
	})

	lines := make([]string, 0, len(page.Assets))
	for _, a := range page.Assets {
		lines = append(lines, fmt.Sprintf(
			"%s — %s (%s) weights: %s",
			a.ID, a.Title, a.Meta[assetcore.MetaCategory], a.Meta[assetcore.MetaWeights],
		))
	}

	return searchResult(fmt.Sprintf("%d font family(-ies) matching %q:", len(page.Assets), query), lines, warns), nil
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

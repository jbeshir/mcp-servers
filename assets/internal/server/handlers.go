package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/fonts"
	"github.com/jbeshir/mcp-servers/assets/internal/icons"
	"github.com/jbeshir/mcp-servers/assets/internal/illustrations"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 200
	defaultFontWeight  = 400
	defaultFontStyle   = "normal"
	cssFormat          = "css"
)

// stringArg reads a string argument, defaulting to "" if absent or of the wrong type.
func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// intArg reads a numeric argument (mcp-go v0.28.0 delivers numbers as float64), defaulting to def
// if absent or of the wrong type.
func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}

	return def
}

// clampLimit applies the default/max bounds shared by every search tool.
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSearchLimit
	case limit > maxSearchLimit:
		return maxSearchLimit
	default:
		return limit
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

// formatList renders a search result header plus one line per result, or a "no matches" note.
func formatList(header string, lines []string) *mcp.CallToolResult {
	if len(lines) == 0 {
		return mcp.NewToolResultText(header + "\n(no matches)")
	}

	return mcp.NewToolResultText(header + "\n" + strings.Join(lines, "\n"))
}

func (s *Server) handleListAssetSources(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var b strings.Builder

	b.WriteString("Icon sets:\n")
	for _, src := range s.catalog.Icons {
		fmt.Fprintf(&b, "  %s — %s — %d icons\n", src.Set, src.License, src.Count)
	}

	b.WriteString("\nIllustration collections:\n")
	for _, src := range s.catalog.Illustrations {
		fmt.Fprintf(&b, "  %s — %s — %d illustrations\n", src.Collection, src.License, src.Count)
	}

	b.WriteString("\nFont families:\n")
	for _, src := range s.catalog.Fonts {
		fmt.Fprintf(&b, "  %s — %s — %s\n", src.Family, src.License, src.Category)
	}

	catalogJSON, err := json.Marshal(s.catalog)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(b.String()),
			mcp.NewTextContent(string(catalogJSON)),
		},
	}, nil
}

func (s *Server) handleSearchIcons(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	set := stringArg(args, "set")
	limit := clampLimit(intArg(args, "limit", 0))

	results := icons.Search(query, set, limit)

	lines := make([]string, 0, len(results))
	for _, m := range results {
		lines = append(lines, fmt.Sprintf("%s/%s", m.Set, m.Name))
	}

	return formatList(fmt.Sprintf("%d icon(s) matching %q:", len(results), query), lines), nil
}

func (s *Server) handleGetIcon(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	set := stringArg(args, "set")
	if set == "" {
		return mcp.NewToolResultError("set is required"), nil
	}

	name := stringArg(args, "name")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	color := stringArg(args, "color")
	size := intArg(args, "size", 0)

	data, err := icons.Render(set, name, color, size)
	if err != nil {
		if errors.Is(err, icons.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("icon not found: %s/%s", set, name)), nil
		}

		return nil, fmt.Errorf("render icon %s/%s: %w", set, name, err)
	}

	license, attribution, _ := s.catalog.IconLicense(set)

	filename := sanitizeFilename(fmt.Sprintf("icon-%s-%s", set, name))
	if size > 0 {
		filename += fmt.Sprintf("-%d", size)
	}

	if color != "" {
		filename += "-" + sanitizeFilename(color)
	}

	filename += ".svg"

	path, err := writeAsset(filename, data)
	if err != nil {
		return nil, fmt.Errorf("write icon %s/%s: %w", set, name, err)
	}

	summary := fmt.Sprintf("Wrote icon %s/%s to %s", set, name, path)
	if strings.EqualFold(set, "simple-icons") {
		summary += ". Note: simple-icons vector data is CC0-1.0, but the brand mark it depicts is a " +
			"third-party trademark — using it must not imply endorsement."
	}

	return newFileResult(summary, []fileEntry{{
		Path:        path,
		Kind:        kindIcon,
		Source:      set,
		License:     license,
		Attribution: attribution,
	}})
}

func (s *Server) handleSearchIllustrations(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	collection := stringArg(args, "collection")
	limit := clampLimit(intArg(args, "limit", 0))

	results := illustrations.Search(query, collection, limit)

	lines := make([]string, 0, len(results))
	for _, m := range results {
		lines = append(lines, fmt.Sprintf("%s/%s", m.Collection, m.Name))
	}

	return formatList(fmt.Sprintf("%d illustration(s) matching %q:", len(results), query), lines), nil
}

func (s *Server) handleGetIllustration(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	collection := stringArg(args, "collection")
	if collection == "" {
		return mcp.NewToolResultError("collection is required"), nil
	}

	name := stringArg(args, "name")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	data, err := illustrations.Get(collection, name)
	if err != nil {
		if errors.Is(err, illustrations.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("illustration not found: %s/%s", collection, name)), nil
		}

		return nil, fmt.Errorf("get illustration %s/%s: %w", collection, name, err)
	}

	license, attribution, _ := s.catalog.IllustrationLicense(collection)

	filename := sanitizeFilename(fmt.Sprintf("illustration-%s-%s", collection, name)) + ".svg"

	path, err := writeAsset(filename, data)
	if err != nil {
		return nil, fmt.Errorf("write illustration %s/%s: %w", collection, name, err)
	}

	return newFileResult(fmt.Sprintf("Wrote illustration %s/%s to %s", collection, name, path), []fileEntry{{
		Path:        path,
		Kind:        kindIllustration,
		Source:      collection,
		License:     license,
		Attribution: attribution,
	}})
}

func (s *Server) handleSearchFonts(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query := stringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	limit := clampLimit(intArg(args, "limit", 0))

	results := fonts.Search(query, limit)

	lines := make([]string, 0, len(results))
	for _, m := range results {
		weights := make([]string, 0, len(m.Weights))
		for _, w := range m.Weights {
			weights = append(weights, strconv.Itoa(w))
		}

		lines = append(lines, fmt.Sprintf("%s (%s) weights: %s", m.Family, m.Category, strings.Join(weights, ",")))
	}

	return formatList(fmt.Sprintf("%d font family(-ies) matching %q:", len(results), query), lines), nil
}

func (s *Server) handleGetFont(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	family := stringArg(args, "family")
	if family == "" {
		return mcp.NewToolResultError("family is required"), nil
	}

	weight := intArg(args, "weight", defaultFontWeight)

	style := stringArg(args, "style")
	if style == "" {
		style = defaultFontStyle
	}

	format := stringArg(args, "format")

	font, err := fonts.Get(family, weight, style)
	if err != nil {
		if errors.Is(err, fonts.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("font not found: %s weight=%d style=%s", family, weight, style)), nil
		}

		return nil, fmt.Errorf("get font %s: %w", family, err)
	}

	license, attribution, _ := s.catalog.FontLicense(family)

	slug := sanitizeFilename(strings.ToLower(family))
	base := fmt.Sprintf("font-%s-%d-%s", slug, weight, style)

	woffPath, err := writeAsset(base+".woff2", font.Data)
	if err != nil {
		return nil, fmt.Errorf("write font %s: %w", family, err)
	}

	files := []fileEntry{{
		Path:        woffPath,
		Kind:        kindFont,
		Source:      family,
		License:     license,
		Attribution: attribution,
	}}

	summary := fmt.Sprintf("Wrote font %s (weight %d, %s) to %s", family, weight, style, woffPath)

	if strings.EqualFold(format, cssFormat) {
		css := fonts.FontFace(family, font)

		cssPath, err := writeAsset(base+".css", []byte(css))
		if err != nil {
			return nil, fmt.Errorf("write font css %s: %w", family, err)
		}

		files = append(files, fileEntry{
			Path:        cssPath,
			Kind:        kindFont,
			Source:      family,
			License:     license,
			Attribution: attribution,
		})
		summary += fmt.Sprintf(" and @font-face CSS to %s", cssPath)
	}

	return newFileResult(summary, files)
}

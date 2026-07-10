package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// testIconSet is the icon set exercised by the get_icon/search_icons handler tests.
const testIconSet = "lucide"

func TestSanitizeFilename(t *testing.T) {
	if got := sanitizeFilename("abc_DEF-123"); got != "abc_DEF-123" {
		t.Errorf("sanitizeFilename(%q) = %q, want unchanged", "abc_DEF-123", got)
	}

	got := sanitizeFilename("a/b\\c..d e")
	for _, bad := range []string{"/", "\\", ".", " "} {
		if strings.Contains(got, bad) {
			t.Errorf("sanitizeFilename(%q) = %q, contains disallowed %q", "a/b\\c..d e", got, bad)
		}
	}
	if want := "a-b-c--d-e"; got != want {
		t.Errorf("sanitizeFilename(%q) = %q, want %q", "a/b\\c..d e", got, want)
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		limit int
		want  int
	}{
		{0, 50},
		{-5, 50},
		{500, 200},
		{100, 100},
		{200, 200},
	}

	for _, tt := range tests {
		if got := clampLimit(tt.limit); got != tt.want {
			t.Errorf("clampLimit(%d) = %d, want %d", tt.limit, got, tt.want)
		}
	}
}

func TestIntArg(t *testing.T) {
	args := map[string]any{
		"num": float64(32),
		"str": "not-a-number",
	}

	if got := intArg(args, "num", 0); got != 32 {
		t.Errorf("intArg(num) = %d, want 32", got)
	}
	if got := intArg(args, "missing", 7); got != 7 {
		t.Errorf("intArg(missing) = %d, want 7", got)
	}
	if got := intArg(args, "str", 9); got != 9 {
		t.Errorf("intArg(str) = %d, want 9", got)
	}
}

func TestStringArg(t *testing.T) {
	args := map[string]any{"key": "value"}

	if got := stringArg(args, "key"); got != "value" {
		t.Errorf("stringArg(key) = %q, want %q", got, "value")
	}
	if got := stringArg(args, "missing"); got != "" {
		t.Errorf("stringArg(missing) = %q, want empty", got)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	deps := config.Setup(config.Config{})

	return NewServer(deps.Registry, deps.Catalog)
}

func newRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	return req
}

func TestHandleGetIconHappyPath(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{
		"set":  testIconSet,
		"name": "a-arrow-down",
	}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("handleGetIcon: result is nil")
	}
	if res.IsError {
		t.Fatalf("handleGetIcon: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if m.Count != 1 || len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: count=%d files=%d", m.Count, len(m.Files))
	}

	entry := m.Files[0]
	if entry.Source != testIconSet {
		t.Errorf("entry.Source = %q, want %q", entry.Source, testIconSet)
	}
	if entry.License != "ISC" {
		t.Errorf("entry.License = %q, want %q", entry.License, "ISC")
	}
	if entry.Kind != kindIcon {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, kindIcon)
	}
	if !strings.HasSuffix(entry.Path, ".svg") {
		t.Errorf("entry.Path = %q, want .svg suffix", entry.Path)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
}

func TestHandleGetIconMissingSet(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{
		"name": "a-arrow-down",
	}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetIcon: expected IsError=true for missing set")
	}
}

func TestHandleGetIconNotFound(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{
		"set":  testIconSet,
		"name": "definitely-not-an-icon",
	}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetIcon: expected IsError=true for unknown icon")
	}
}

func TestHandleSearchIconsHappyPath(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleSearchIcons(t.Context(), newRequest(map[string]any{
		"query": "arrow",
	}))
	if err != nil {
		t.Fatalf("handleSearchIcons: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleSearchIcons: result is an error: %+v", res.Content)
	}

	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want mcp.TextContent", res.Content[0])
	}
	if strings.Contains(text.Text, "no matches") {
		t.Errorf("handleSearchIcons text = %q, want at least one match", text.Text)
	}
}

func TestHandleGetFontCSS(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetFont(t.Context(), newRequest(map[string]any{
		"family": "Inter",
		"weight": float64(400),
		"format": "css",
	}))
	if err != nil {
		t.Fatalf("handleGetFont: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetFont: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if m.Count != 2 {
		t.Fatalf("manifest count = %d, want 2 (woff2 + css)", m.Count)
	}

	exts := make(map[string]bool, len(m.Files))
	for _, entry := range m.Files {
		if entry.Kind != kindFont {
			t.Errorf("entry.Kind = %q, want %q", entry.Kind, kindFont)
		}
		if _, err := os.Stat(entry.Path); err != nil {
			t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
		}
		exts[filepath.Ext(entry.Path)] = true
	}
	if !exts[".woff2"] {
		t.Errorf("manifest files = %+v, missing .woff2", m.Files)
	}
	if !exts[".css"] {
		t.Errorf("manifest files = %+v, missing .css", m.Files)
	}
}

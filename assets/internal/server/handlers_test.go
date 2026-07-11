package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// Composite ids exercised by the handler tests.
const (
	testIconID  = "embedded-icons:lucide/a-arrow-down"
	testIconSet = "lucide"
	testFontID  = "embedded-fonts:inter"
)

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

func TestStringSliceArg(t *testing.T) {
	args := map[string]any{
		"list":  []any{"a", "", "b", 1, "c"},
		"wrong": "not-a-list",
	}

	got := stringSliceArg(args, "list")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("stringSliceArg(list) = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("stringSliceArg(list)[%d] = %q, want %q", i, got[i], w)
		}
	}
	if got := stringSliceArg(args, "wrong"); got != nil {
		t.Errorf("stringSliceArg(wrong) = %v, want nil", got)
	}
	if got := stringSliceArg(args, "missing"); got != nil {
		t.Errorf("stringSliceArg(missing) = %v, want nil", got)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	deps := config.Setup(config.Config{})

	return NewServer(deps.Registry)
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
		"id": testIconID,
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

func TestHandleGetIconMissingID(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetIcon: expected IsError=true for missing id")
	}
}

func TestHandleGetIconNotFound(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{
		"id": "embedded-icons:lucide/definitely-not-an-icon",
	}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetIcon: expected IsError=true for unknown icon")
	}
}

func TestHandleGetIconUnknownProvider(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIcon(t.Context(), newRequest(map[string]any{
		"id": "no-such-provider:lucide/a-arrow-down",
	}))
	if err != nil {
		t.Fatalf("handleGetIcon: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetIcon: expected IsError=true for unknown provider")
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
	// Each hit must carry its composite id so the caller can round-trip it to get_icon.
	if !strings.Contains(text.Text, "embedded-icons:") {
		t.Errorf("handleSearchIcons text = %q, want composite ids", text.Text)
	}
}

func TestHandleSearchIconsSourceFilter(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleSearchIcons(t.Context(), newRequest(map[string]any{
		"query":   "arrow",
		"sources": []any{"lucide"},
	}))
	if err != nil {
		t.Fatalf("handleSearchIcons: unexpected error: %v", err)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "no matches") {
		t.Fatalf("source-scoped search returned no matches: %q", text)
	}
	// Scoped to lucide: no other set's composite id should appear.
	for _, other := range []string{"embedded-icons:tabler/", "embedded-icons:feather/", "embedded-icons:phosphor/"} {
		if strings.Contains(text, other) {
			t.Errorf("source-scoped search leaked %q: %s", other, text)
		}
	}
}

func TestHandleGetFontCSS(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetFont(t.Context(), newRequest(map[string]any{
		"id":     testFontID,
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

func TestHandleGetIllustration(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())
	s := newTestServer(t)

	res, err := s.handleGetIllustration(t.Context(), newRequest(map[string]any{
		"id": "embedded-illustrations:open-doodles/ballet-doodle",
	}))
	if err != nil {
		t.Fatalf("handleGetIllustration: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetIllustration: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if m.Count != 1 || m.Files[0].Kind != kindIllustration {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if m.Files[0].License != "CC0-1.0" {
		t.Errorf("illustration license = %q, want CC0-1.0", m.Files[0].License)
	}
}

func TestHandleListAssetSources(t *testing.T) {
	s := newTestServer(t)

	res, err := s.handleListAssetSources(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleListAssetSources: unexpected error: %v", err)
	}
	if len(res.Content) != 2 {
		t.Fatalf("content length = %d, want 2 (listing + JSON)", len(res.Content))
	}

	listing := res.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"embedded-icons", "embedded-illustrations", "embedded-fonts", "lucide", "ISC"} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing missing %q:\n%s", want, listing)
		}
	}

	structured := res.Content[1].(mcp.TextContent).Text
	if !strings.Contains(structured, `"providers"`) {
		t.Errorf("structured block missing providers key: %s", structured)
	}
}

func TestHandleListAssetSourcesKindFilter(t *testing.T) {
	s := newTestServer(t)

	res, err := s.handleListAssetSources(t.Context(), newRequest(map[string]any{
		"kind": "font",
	}))
	if err != nil {
		t.Fatalf("handleListAssetSources: unexpected error: %v", err)
	}

	listing := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(listing, "embedded-fonts") {
		t.Errorf("kind=font listing missing embedded-fonts:\n%s", listing)
	}
	if strings.Contains(listing, "embedded-icons") || strings.Contains(listing, "embedded-illustrations") {
		t.Errorf("kind=font listing leaked a non-font provider:\n%s", listing)
	}
}

func TestHandleListAssetSourcesSourceFilter(t *testing.T) {
	s := newTestServer(t)

	res, err := s.handleListAssetSources(t.Context(), newRequest(map[string]any{
		"sources": []any{"lucide"},
	}))
	if err != nil {
		t.Fatalf("handleListAssetSources: unexpected error: %v", err)
	}

	listing := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(listing, "lucide") {
		t.Errorf("source=lucide listing missing lucide:\n%s", listing)
	}
	// Providers whose sources are all filtered out are omitted, so only embedded-icons should appear.
	if strings.Contains(listing, "embedded-fonts") || strings.Contains(listing, "embedded-illustrations") {
		t.Errorf("source=lucide listing leaked another provider:\n%s", listing)
	}
}

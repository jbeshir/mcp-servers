package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
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

// newTestServer builds a Server wired to a fresh registry, writing rendered assets under a
// per-test temp directory.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	deps := config.Setup(config.Config{OutputDir: t.TempDir()})

	return NewServer(deps.Registry, deps.OutputDir)
}

func newRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	return req
}

func TestHandleGetIconHappyPath(t *testing.T) {
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
	if len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: files=%d", len(m.Files))
	}

	assertManifestFileShape(t, m.Files[0], testIconID, string(assetcore.KindIcon), testIconSet, "ISC")
	if !strings.HasSuffix(m.Files[0].Path, ".svg") {
		t.Errorf("entry.Path = %q, want .svg suffix", m.Files[0].Path)
	}
}

// assertManifestFileShape asserts the structuredContent.files[N] shape the demesne filegen adapter and
// other MCP clients depend on: path, id, kind, source, title, and an embedded license.spdx, plus that
// the file was actually written to path.
func assertManifestFileShape(t *testing.T, entry manifestFile, wantID, wantKind, wantSource, wantSPDX string) {
	t.Helper()

	if entry.Path == "" {
		t.Error("entry.Path is empty")
	}
	if entry.ID != wantID {
		t.Errorf("entry.ID = %q, want %q", entry.ID, wantID)
	}
	if entry.Kind != wantKind {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, wantKind)
	}
	if entry.Source != wantSource {
		t.Errorf("entry.Source = %q, want %q", entry.Source, wantSource)
	}
	if entry.Title == "" {
		t.Error("entry.Title is empty")
	}
	if entry.License.SPDX != wantSPDX {
		t.Errorf("entry.License.SPDX = %q, want %q", entry.License.SPDX, wantSPDX)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
}

func TestHandleGetIconMissingID(t *testing.T) {
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
	if len(m.Files) != 2 {
		t.Fatalf("manifest files = %d, want 2 (woff2 + css)", len(m.Files))
	}

	exts := make(map[string]bool, len(m.Files))
	for _, entry := range m.Files {
		assertFontManifestFile(t, entry)
		exts[filepath.Ext(entry.Path)] = true
	}
	if !exts[".woff2"] {
		t.Errorf("manifest files = %+v, missing .woff2", m.Files)
	}
	if !exts[".css"] {
		t.Errorf("manifest files = %+v, missing .css", m.Files)
	}
}

// assertFontManifestFile asserts the common shape of one get_font manifest entry, plus that a css
// entry (identified by its path extension) carries the text/css content type.
func assertFontManifestFile(t *testing.T, entry manifestFile) {
	t.Helper()

	if entry.Kind != string(assetcore.KindFont) {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, assetcore.KindFont)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
	if filepath.Ext(entry.Path) == ".css" && entry.ContentType != "text/css" {
		t.Errorf("css entry.ContentType = %q, want text/css", entry.ContentType)
	}
}

func TestHandleGetIllustration(t *testing.T) {
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
	if len(m.Files) != 1 || m.Files[0].Kind != string(assetcore.KindIllustration) {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if m.Files[0].License.SPDX != "CC0-1.0" {
		t.Errorf("illustration license.spdx = %q, want CC0-1.0", m.Files[0].License.SPDX)
	}
}

func TestHandleListAssetSources(t *testing.T) {
	s := newTestServer(t)

	res, err := s.handleListAssetSources(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleListAssetSources: unexpected error: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("content length = %d, want 1 (human summary only)", len(res.Content))
	}

	listing := res.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"embedded-icons", "embedded-illustrations", "embedded-fonts", "lucide", "ISC"} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing missing %q:\n%s", want, listing)
		}
	}

	m, ok := res.StructuredContent.(providersManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want providersManifest", res.StructuredContent)
	}
	if len(m.Providers) == 0 {
		t.Fatal("providersManifest.Providers is empty")
	}

	names := make([]string, 0, len(m.Providers))
	for _, p := range m.Providers {
		names = append(names, p.Provider)
	}
	for _, want := range []string{"embedded-icons", "embedded-illustrations", "embedded-fonts"} {
		if !strings.Contains(strings.Join(names, ","), want) {
			t.Errorf("providers %v missing %q", names, want)
		}
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

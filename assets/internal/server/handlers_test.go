package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
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

func newAssetsDBTestServer(t *testing.T) (*Server, []byte, string) {
	t.Helper()
	dbDir := t.TempDir()
	itemPath := "sprites/sheet.png"

	// #nosec G304 -- the path is a fixed test fixture beneath t.TempDir.
	dataPackage, err := os.Create(filepath.Join(dbDir, "datapackage.json"))
	require.NoError(t, err)

	index := map[string]any{
		"name": "fixture", "title": "Fixture", "version": "1",
		"created": "2026-07-18T00:00:00Z", "x_assetsdb:schemaVersion": 1,
		"x_assetsdb:sources": []any{map[string]any{"name": "tiny-pack", "title": "Tiny Pack", "path": "sources/tiny-pack.zip", "licenses": []any{map[string]any{"name": "CC0-1.0", "title": "CC Zero"}}}},
		"resources": []any{
			map[string]any{"name": "hero", "title": "Hero", "x_assetsdb:id": "assetsdb:tiny-pack/sprites/sheet.png#hero", "x_assetsdb:source": "tiny-pack", "x_assetsdb:kind": "sprite2d", "path": itemPath, "mediatype": "image/png", "x_assetsdb:region": map[string]any{"x": 1, "y": 2, "width": 8, "height": 9}},
			map[string]any{"name": "door", "title": "Door", "x_assetsdb:id": "assetsdb:tiny-pack/sprites/sheet.png#door", "x_assetsdb:source": "tiny-pack", "x_assetsdb:kind": "sprite2d", "path": itemPath, "mediatype": "image/png", "x_assetsdb:region": map[string]any{"x": 9, "y": 2, "width": 8, "height": 9}},
		},
	}
	require.NoError(t, json.NewEncoder(dataPackage).Encode(index))
	require.NoError(t, dataPackage.Close())

	require.NoError(t, os.MkdirAll(filepath.Join(dbDir, "sources"), 0o750))

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(itemPath)
	require.NoError(t, err)
	_, err = w.Write([]byte("sprite-bytes"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zipPath := filepath.Join(dbDir, filepath.FromSlash("sources/tiny-pack.zip"))
	require.NoError(t, os.WriteFile(zipPath, buf.Bytes(), 0o600))

	deps := config.Setup(config.Config{AssetsDB: dbDir, OutputDir: t.TempDir(), DisableRemote: true})

	return NewServer(deps.Registry, deps.OutputDir, deps.PackStore), buf.Bytes(), zipPath
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

	return NewServer(deps.Registry, deps.OutputDir, deps.PackStore)
}

func newRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	return req
}

func TestToolDiscoveryExplainsLexicalSearchAndPackListing(t *testing.T) {
	s := newTestServer(t)
	response := s.mcpServer.HandleMessage(t.Context(), []byte(`{
		"jsonrpc":"2.0","id":1,"method":"tools/list"
	}`))
	jsonResponse, ok := response.(mcp.JSONRPCResponse)
	require.True(t, ok)
	result, ok := jsonResponse.Result.(mcp.ListToolsResult)
	require.True(t, ok)

	var packTool *mcp.Tool
	for i := range result.Tools {
		tool := &result.Tools[i]
		if strings.HasPrefix(tool.Name, "search_") {
			require.Contains(t, strings.ToLower(tool.Description), "not vector", tool.Name)
			require.Contains(t, strings.ToLower(tool.Description), "literal", tool.Name)
		}
		if tool.Name == "list_pack_assets" {
			packTool = tool
		}
	}
	require.NotNil(t, packTool)
	require.ElementsMatch(t, []string{"pack_id"}, packTool.InputSchema.Required)
	for _, name := range []string{"pack_id", "kind", "limit", "cursor"} {
		require.Contains(t, packTool.InputSchema.Properties, name)
	}
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

func TestHandleSearchAndGetSpriteFromAssetsDB(t *testing.T) {
	s, _, _ := newAssetsDBTestServer(t)
	res, err := s.handleSearchSprites(t.Context(), newRequest(map[string]any{"query": "hero"}))
	require.NoError(t, err)
	require.False(t, res.IsError)
	listing := res.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"assetsdb:tiny-pack/sprites/sheet.png#hero", `pack=tiny-pack`, `pack_title="Tiny Pack"`, "region=1,2,8,9"} {
		require.Contains(t, listing, want)
	}

	res, err = s.handleGetSprite(t.Context(), newRequest(map[string]any{"id": "assetsdb:tiny-pack/sprites/sheet.png#hero"}))
	require.NoError(t, err)
	require.False(t, res.IsError)
	m := res.StructuredContent.(fileManifest)
	require.Len(t, m.Files, 1)
	assertManifestFileShape(t, m.Files[0], "assetsdb:tiny-pack/sprites/sheet.png#hero", string(assetcore.KindSprite), "tiny-pack", "CC0-1.0")
	got, err := os.ReadFile(m.Files[0].Path)
	require.NoError(t, err)
	require.Equal(t, []byte("sprite-bytes"), got)
}

func TestHandleListPackAssetsPaginatesAndPreservesRegionMetadata(t *testing.T) {
	s, _, _ := newAssetsDBTestServer(t)
	res, err := s.handleListPackAssets(t.Context(), newRequest(map[string]any{
		"pack_id": "tiny-pack", "kind": "sprite", "limit": float64(1),
	}))
	require.NoError(t, err)
	require.False(t, res.IsError)
	listing := res.Content[0].(mcp.TextContent).Text
	require.Contains(t, listing, "assetsdb:tiny-pack/sprites/sheet.png#")
	require.Contains(t, listing, "region=")
	require.Contains(t, listing, "next_cursor: 1")

	res, err = s.handleListPackAssets(t.Context(), newRequest(map[string]any{
		"pack_id": "tiny-pack", "kind": "sprite", "limit": float64(1), "cursor": "1",
	}))
	require.NoError(t, err)
	require.False(t, res.IsError)
	listing = res.Content[0].(mcp.TextContent).Text
	require.NotContains(t, listing, "next_cursor:")
}

func TestHandleListPackAssetsErrors(t *testing.T) {
	s, _, _ := newAssetsDBTestServer(t)
	for name, args := range map[string]map[string]any{
		"missing pack":   {},
		"unknown pack":   {"pack_id": "unknown"},
		"invalid kind":   {"pack_id": "tiny-pack", "kind": "photo"},
		"invalid cursor": {"pack_id": "tiny-pack", "cursor": "later"},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := s.handleListPackAssets(t.Context(), newRequest(args))
			require.NoError(t, err)
			require.True(t, res.IsError)
		})
	}
}

func TestHandleGetPackWritesByteIdenticalInspectableZIP(t *testing.T) {
	s, original, _ := newAssetsDBTestServer(t)
	res, err := s.handleGetPack(t.Context(), newRequest(map[string]any{"pack_id": "tiny-pack"}))
	require.NoError(t, err)
	require.False(t, res.IsError)
	m := res.StructuredContent.(fileManifest)
	require.Len(t, m.Files, 1)
	require.Equal(t, "application/zip", m.Files[0].ContentType)
	require.Equal(t, "CC0-1.0", m.Files[0].License.SPDX)
	got, err := os.ReadFile(m.Files[0].Path)
	require.NoError(t, err)
	require.Equal(t, original, got)
	zr, err := zip.NewReader(bytes.NewReader(got), int64(len(got)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	require.Equal(t, "sprites/sheet.png", zr.File[0].Name)
}

func TestHandleGetPackErrors(t *testing.T) {
	s, _, zipPath := newAssetsDBTestServer(t)
	for name, args := range map[string]map[string]any{
		"missing argument": {},
		"unknown pack":     {"pack_id": "unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := s.handleGetPack(t.Context(), newRequest(args))
			require.NoError(t, err)
			require.True(t, res.IsError)
		})
	}
	require.NoError(t, os.Remove(zipPath))
	res, err := s.handleGetPack(t.Context(), newRequest(map[string]any{"pack_id": "tiny-pack"}))
	require.NoError(t, err)
	require.True(t, res.IsError)

	unconfigured := NewServer(assetcore.NewRegistry(), t.TempDir(), nil)
	res, err = unconfigured.handleGetPack(t.Context(), newRequest(map[string]any{"pack_id": "tiny-pack"}))
	require.NoError(t, err)
	require.True(t, res.IsError)
}

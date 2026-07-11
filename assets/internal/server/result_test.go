package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestWriteAssetAbsolute(t *testing.T) {
	dir := t.TempDir()

	path, err := writeAsset(dir, "sample.svg", []byte("<svg/>"))
	if err != nil {
		t.Fatalf("writeAsset: unexpected error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("writeAsset path %q is not absolute", path)
	}
	if filepath.Base(path) != "sample.svg" {
		t.Errorf("writeAsset base = %q, want %q", filepath.Base(path), "sample.svg")
	}

	data, err := os.ReadFile(path) //nolint:gosec // test reads back a file it just wrote under t.TempDir()
	if err != nil {
		t.Fatalf("read written asset: %v", err)
	}
	if string(data) != "<svg/>" {
		t.Errorf("written asset content = %q, want %q", string(data), "<svg/>")
	}
}

func TestWriteAssetCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")

	if _, err := writeAsset(dir, "sample.svg", []byte("<svg/>")); err != nil {
		t.Fatalf("writeAsset: unexpected error: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("output dir does not exist: %v", err)
	}
}

func testBlob() assetcore.Blob {
	return assetcore.Blob{
		Asset: assetcore.Asset{
			ID:     "embedded-icons:lucide/camera",
			Kind:   assetcore.KindIcon,
			Source: "lucide",
			Title:  "camera",
			License: assetcore.License{
				SPDX: "ISC",
			},
		},
		Content:     []byte("<svg/>"),
		ContentType: "image/svg+xml",
	}
}

func TestManifestFileForShape(t *testing.T) {
	blob := testBlob()

	entry := manifestFileFor("/tmp/x.svg", blob)

	if entry.Path != "/tmp/x.svg" {
		t.Errorf("entry.Path = %q, want /tmp/x.svg", entry.Path)
	}
	if entry.ID != blob.Asset.ID {
		t.Errorf("entry.ID = %q, want %q", entry.ID, blob.Asset.ID)
	}
	if entry.Kind != string(blob.Asset.Kind) {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, string(blob.Asset.Kind))
	}
	if entry.Source != blob.Asset.Source {
		t.Errorf("entry.Source = %q, want %q", entry.Source, blob.Asset.Source)
	}
	if entry.Title != blob.Asset.Title {
		t.Errorf("entry.Title = %q, want %q", entry.Title, blob.Asset.Title)
	}
	if entry.ContentType != blob.ContentType {
		t.Errorf("entry.ContentType = %q, want %q", entry.ContentType, blob.ContentType)
	}
	if entry.License != blob.Asset.License {
		t.Errorf("entry.License = %+v, want %+v", entry.License, blob.Asset.License)
	}
}

func TestNewFileResultShape(t *testing.T) {
	want := manifestFileFor("/tmp/x.svg", testBlob())

	res, err := newFileResult("wrote 1 file", []manifestFile{want})
	if err != nil {
		t.Fatalf("newFileResult: unexpected error: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("res.Content has length %d, want 1", len(res.Content))
	}

	summary, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want mcp.TextContent", res.Content[0])
	}
	if summary.Text != "wrote 1 file" {
		t.Errorf("summary text = %q, want %q", summary.Text, "wrote 1 file")
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: files=%d", len(m.Files))
	}
	if m.Files[0] != want {
		t.Errorf("manifest.Files[0] = %+v, want %+v", m.Files[0], want)
	}
}

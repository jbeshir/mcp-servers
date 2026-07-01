package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

const envOutputDir = "ASSETS_OUTPUT_DIR"

func TestOutputDirHonorsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envOutputDir, dir)

	got, err := outputDir()
	if err != nil {
		t.Fatalf("outputDir: unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("outputDir() = %q, want %q", got, dir)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("output dir does not exist: %v", err)
	}
}

func TestOutputDirDefault(t *testing.T) {
	t.Setenv(envOutputDir, "")

	got, err := outputDir()
	if err != nil {
		t.Fatalf("outputDir: unexpected error: %v", err)
	}
	want := filepath.Join(os.TempDir(), "assets-mcp")
	if got != want {
		t.Errorf("outputDir() = %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("output dir does not exist: %v", err)
	}
}

func TestWriteAssetAbsolute(t *testing.T) {
	t.Setenv(envOutputDir, t.TempDir())

	path, err := writeAsset("sample.svg", []byte("<svg/>"))
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

func TestNewFileResultShape(t *testing.T) {
	want := fileEntry{Path: "/tmp/x.svg", Kind: kindIcon, Source: "lucide", License: "ISC", Attribution: ""}

	res, err := newFileResult("wrote 1 file", []fileEntry{want})
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
	if m.Count != 1 || len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: count=%d files=%d", m.Count, len(m.Files))
	}
	if m.Count != len(m.Files) {
		t.Errorf("manifest count %d != len(files) %d", m.Count, len(m.Files))
	}
	if m.Files[0] != want {
		t.Errorf("manifest.Files[0] = %+v, want %+v", m.Files[0], want)
	}
}

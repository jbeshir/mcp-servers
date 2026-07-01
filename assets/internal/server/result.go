package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	kindIcon         = "icon"
	kindIllustration = "illustration"
	kindFont         = "font"
)

// fileEntry describes a single asset file written to disk by a tool call.
type fileEntry struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Source      string `json:"source"`
	License     string `json:"license"`
	Attribution string `json:"attribution"`
}

// fileManifest is the JSON shape emitted as native structured content by file-producing tools.
type fileManifest struct {
	Files []fileEntry `json:"files"`
	Count int         `json:"count"`
}

// outputDir returns the directory rendered assets are written to: ASSETS_OUTPUT_DIR if set,
// otherwise a subdirectory of the OS temp dir. The directory is created if it does not exist.
func outputDir() (string, error) {
	dir := os.Getenv("ASSETS_OUTPUT_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "assets-mcp")
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	return dir, nil
}

// writeAsset writes data to filename (sanitized to its base name) under the output directory and
// returns the absolute path written.
func writeAsset(filename string, data []byte) (string, error) {
	dir, err := outputDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, filepath.Base(filename))

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write asset %s: %w", filename, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path for %s: %w", filename, err)
	}

	return abs, nil
}

// newFileResult builds a CallToolResult for a file-producing tool: a human-readable summary text
// block plus the machine-readable file manifest as native structured content.
//
// The manifest is carried in result.StructuredContent (mcp-go's native structured output), shaped
// {"files":[{path,kind,source,license,attribution}],"count":N}; the summary text block is retained
// so clients that ignore structured content still get a readable result.
func newFileResult(summary string, files []fileEntry) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(summary),
		},
		StructuredContent: fileManifest{Files: files, Count: len(files)},
	}, nil
}

package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/mcp"
)

// manifestFile describes a single asset file written to disk by a tool call, derived entirely from
// the assetcore.Blob it was rendered from.
type manifestFile struct {
	Path        string            `json:"path"`
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Source      string            `json:"source"`
	Title       string            `json:"title"`
	ContentType string            `json:"content_type,omitempty"`
	License     assetcore.License `json:"license"`
}

// manifestFileFor builds a manifestFile for path from b's asset metadata and content type.
func manifestFileFor(path string, b assetcore.Blob) manifestFile {
	return manifestFile{
		Path:        path,
		ID:          b.Asset.ID,
		Kind:        string(b.Asset.Kind),
		Source:      b.Asset.Source,
		Title:       b.Asset.Title,
		ContentType: b.ContentType,
		License:     b.Asset.License,
	}
}

// fileManifest is the JSON shape emitted as native structured content by file-producing tools.
type fileManifest struct {
	Files []manifestFile `json:"files"`
}

// writeAsset writes data to filename (sanitized to its base name) under dir, creating dir if it does
// not yet exist, and returns the absolute path written.
func writeAsset(dir, filename string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
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
// {"files":[manifestFile,...]}; the summary text block is retained so clients that ignore structured
// content still get a readable result.
func newFileResult(summary string, files []manifestFile) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(summary),
		},
		StructuredContent: fileManifest{Files: files},
	}, nil
}

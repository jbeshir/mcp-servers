package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore/mocks"
	"github.com/mark3labs/mcp-go/mcp"
)

// testModelProviderName is the provider name used by the mock-backed model handler tests. These
// tests register a generated mock directly against a fresh registry, exercising the registry/handler
// plumbing rather than the embedded providers newTestServer wires up.
const testModelProviderName = "mock-models"

// newModelServer builds a Server backed by a fresh registry holding a single mock ModelProvider,
// registered under its Name.
func newModelServer(t *testing.T) (*Server, *mocks.ModelProvider) {
	t.Helper()

	modelProv := mocks.NewModelProvider(t)
	modelProv.EXPECT().Name().Return(testModelProviderName)

	registry := assetcore.NewRegistry()
	registry.AddModel(modelProv)

	return NewServer(registry, t.TempDir()), modelProv
}

func TestHandleSearchModelsHappyPath(t *testing.T) {
	s, modelProv := newModelServer(t)

	id := testModelProviderName + ":chair"
	modelProv.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{
		Assets:     []assetcore.Asset{{ID: id, Source: testModelProviderName, Title: "Chair"}},
		NextCursor: "more",
	}, nil)

	res, err := s.handleSearchModels(t.Context(), newRequest(map[string]any{
		"query": "chair",
	}))
	if err != nil {
		t.Fatalf("handleSearchModels: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleSearchModels: result is an error: %+v", res.Content)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, id) {
		t.Errorf("handleSearchModels text = %q, want composite id %q", text, id)
	}
	if !strings.Contains(text, "next_cursor:") {
		t.Errorf("handleSearchModels text = %q, want a next_cursor line", text)
	}
}

func TestHandleSearchModelsMissingQuery(t *testing.T) {
	s, _ := newModelServer(t)

	res, err := s.handleSearchModels(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleSearchModels: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleSearchModels: expected IsError=true for missing query")
	}
}

func TestHandleGetModelHappyPath(t *testing.T) {
	s, modelProv := newModelServer(t)

	id := testModelProviderName + ":chair"
	modelProv.EXPECT().Fetch(mock.Anything, "chair", assetcore.ModelFetchOpts{}).Return(assetcore.Blob{
		Asset: assetcore.Asset{
			ID: id, Kind: assetcore.KindModel, Source: testModelProviderName, Title: "Chair",
			License: assetcore.License{SPDX: "CC0-1.0"},
		},
		Content:     []byte("fake-glb-bytes"),
		Filename:    "chair.glb",
		ContentType: "model/gltf-binary",
	}, nil)

	res, err := s.handleGetModel(t.Context(), newRequest(map[string]any{
		"id": id,
	}))
	if err != nil {
		t.Fatalf("handleGetModel: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetModel: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: files=%d", len(m.Files))
	}

	entry := m.Files[0]
	if entry.Kind != string(assetcore.KindModel) {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, assetcore.KindModel)
	}
	if !strings.HasPrefix(filepath.Base(entry.Path), "model-") {
		t.Errorf("entry.Path base = %q, want model- prefix", filepath.Base(entry.Path))
	}
	if !strings.HasSuffix(entry.Path, ".glb") {
		t.Errorf("entry.Path = %q, want .glb suffix", entry.Path)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
}

func TestHandleGetModelMissingID(t *testing.T) {
	s, _ := newModelServer(t)

	res, err := s.handleGetModel(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleGetModel: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetModel: expected IsError=true for missing id")
	}
}

func TestHandleGetModelNotFound(t *testing.T) {
	s, modelProv := newModelServer(t)

	modelProv.EXPECT().Fetch(mock.Anything, "missing", assetcore.ModelFetchOpts{}).
		Return(assetcore.Blob{}, assetcore.ErrNotFound)

	res, err := s.handleGetModel(t.Context(), newRequest(map[string]any{
		"id": testModelProviderName + ":missing",
	}))
	if err != nil {
		t.Fatalf("handleGetModel: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetModel: expected IsError=true for unknown model")
	}
}

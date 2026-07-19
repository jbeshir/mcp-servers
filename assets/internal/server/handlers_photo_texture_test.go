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

// Provider names used by the mock-backed photo/texture handler tests. These tests register generated
// mocks directly against a fresh registry, exercising the registry/handler plumbing rather than the
// embedded providers newTestServer wires up.
const (
	testPhotoProviderName   = "mock-photos"
	testTextureProviderName = "mock-textures"
)

// newPhotoTextureServer builds a Server backed by a fresh registry holding a single mock
// PhotoProvider and TextureProvider, each registered under its Name.
func newPhotoTextureServer(t *testing.T) (*Server, *mocks.PhotoProvider, *mocks.TextureProvider) {
	t.Helper()

	photoProv := mocks.NewPhotoProvider(t)
	photoProv.EXPECT().Name().Return(testPhotoProviderName)

	textureProv := mocks.NewTextureProvider(t)
	textureProv.EXPECT().Name().Return(testTextureProviderName)

	registry := assetcore.NewRegistry()
	registry.AddPhoto(photoProv)
	registry.AddTexture(textureProv)

	return NewServer(registry, t.TempDir(), nil), photoProv, textureProv
}

func TestHandleSearchPhotosHappyPath(t *testing.T) {
	s, photoProv, _ := newPhotoTextureServer(t)

	id := testPhotoProviderName + ":sunset"
	photoProv.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{
		Assets:     []assetcore.Asset{{ID: id, Source: testPhotoProviderName, Title: "Sunset"}},
		NextCursor: "more",
	}, nil)

	res, err := s.handleSearchPhotos(t.Context(), newRequest(map[string]any{
		"query": "sunset",
	}))
	if err != nil {
		t.Fatalf("handleSearchPhotos: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleSearchPhotos: result is an error: %+v", res.Content)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, id) {
		t.Errorf("handleSearchPhotos text = %q, want composite id %q", text, id)
	}
	if !strings.Contains(text, "next_cursor:") {
		t.Errorf("handleSearchPhotos text = %q, want a next_cursor line", text)
	}
}

func TestHandleSearchPhotosMissingQuery(t *testing.T) {
	s, _, _ := newPhotoTextureServer(t)

	res, err := s.handleSearchPhotos(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleSearchPhotos: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleSearchPhotos: expected IsError=true for missing query")
	}
}

func TestHandleGetPhotoHappyPath(t *testing.T) {
	s, photoProv, _ := newPhotoTextureServer(t)

	id := testPhotoProviderName + ":sunset"
	photoProv.EXPECT().Fetch(mock.Anything, "sunset", assetcore.PhotoFetchOpts{}).Return(assetcore.Blob{
		Asset: assetcore.Asset{
			ID: id, Kind: assetcore.KindPhoto, Source: testPhotoProviderName, Title: "Sunset",
			License: assetcore.License{SPDX: "CC0-1.0"},
		},
		Content:     []byte("fake-jpeg-bytes"),
		Filename:    "sunset.jpg",
		ContentType: "image/jpeg",
	}, nil)

	res, err := s.handleGetPhoto(t.Context(), newRequest(map[string]any{
		"id": id,
	}))
	if err != nil {
		t.Fatalf("handleGetPhoto: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetPhoto: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: files=%d", len(m.Files))
	}

	entry := m.Files[0]
	if entry.Kind != string(assetcore.KindPhoto) {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, assetcore.KindPhoto)
	}
	if !strings.HasPrefix(filepath.Base(entry.Path), "photo-") {
		t.Errorf("entry.Path base = %q, want photo- prefix", filepath.Base(entry.Path))
	}
	if !strings.HasSuffix(entry.Path, ".jpg") {
		t.Errorf("entry.Path = %q, want .jpg suffix", entry.Path)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
}

func TestHandleGetPhotoMissingID(t *testing.T) {
	s, _, _ := newPhotoTextureServer(t)

	res, err := s.handleGetPhoto(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleGetPhoto: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetPhoto: expected IsError=true for missing id")
	}
}

func TestHandleGetPhotoNotFound(t *testing.T) {
	s, photoProv, _ := newPhotoTextureServer(t)

	photoProv.EXPECT().Fetch(mock.Anything, "missing", assetcore.PhotoFetchOpts{}).
		Return(assetcore.Blob{}, assetcore.ErrNotFound)

	res, err := s.handleGetPhoto(t.Context(), newRequest(map[string]any{
		"id": testPhotoProviderName + ":missing",
	}))
	if err != nil {
		t.Fatalf("handleGetPhoto: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetPhoto: expected IsError=true for unknown photo")
	}
}

func TestHandleSearchTexturesHappyPath(t *testing.T) {
	s, _, textureProv := newPhotoTextureServer(t)

	id := testTextureProviderName + ":brick-wall"
	textureProv.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{
		Assets: []assetcore.Asset{{ID: id, Source: testTextureProviderName, Title: "Brick Wall"}},
	}, nil)

	res, err := s.handleSearchTextures(t.Context(), newRequest(map[string]any{
		"query": "brick",
	}))
	if err != nil {
		t.Fatalf("handleSearchTextures: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleSearchTextures: result is an error: %+v", res.Content)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, id) {
		t.Errorf("handleSearchTextures text = %q, want composite id %q", text, id)
	}
}

func TestHandleSearchTexturesMissingQuery(t *testing.T) {
	s, _, _ := newPhotoTextureServer(t)

	res, err := s.handleSearchTextures(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleSearchTextures: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleSearchTextures: expected IsError=true for missing query")
	}
}

func TestHandleGetTextureHappyPath(t *testing.T) {
	s, _, textureProv := newPhotoTextureServer(t)

	id := testTextureProviderName + ":brick-wall"
	textureProv.EXPECT().
		Fetch(mock.Anything, "brick-wall", assetcore.TextureFetchOpts{Resolution: defaultTextureResolution, Format: defaultTextureFormat}).
		Return(assetcore.Blob{
			Asset: assetcore.Asset{
				ID: id, Kind: assetcore.KindTexture, Source: testTextureProviderName, Title: "Brick Wall",
				License: assetcore.License{SPDX: "CC0-1.0"},
			},
			Content:     []byte("fake-zip-bytes"),
			Filename:    "brick-wall_1K-JPG.zip",
			ContentType: "application/zip",
		}, nil)

	res, err := s.handleGetTexture(t.Context(), newRequest(map[string]any{
		"id": id,
	}))
	if err != nil {
		t.Fatalf("handleGetTexture: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetTexture: result is an error: %+v", res.Content)
	}

	m, ok := res.StructuredContent.(fileManifest)
	if !ok {
		t.Fatalf("StructuredContent is %T, want fileManifest", res.StructuredContent)
	}
	if len(m.Files) != 1 {
		t.Fatalf("unexpected manifest: files=%d", len(m.Files))
	}

	entry := m.Files[0]
	if entry.Kind != string(assetcore.KindTexture) {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, assetcore.KindTexture)
	}
	if !strings.HasSuffix(entry.Path, ".zip") {
		t.Errorf("entry.Path = %q, want .zip suffix", entry.Path)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("entry.Path %q does not exist: %v", entry.Path, err)
	}
}

func TestHandleGetTextureCustomResolutionFormat(t *testing.T) {
	s, _, textureProv := newPhotoTextureServer(t)

	id := testTextureProviderName + ":brick-wall"
	textureProv.EXPECT().
		Fetch(mock.Anything, "brick-wall", assetcore.TextureFetchOpts{Resolution: "4K", Format: "PNG"}).
		Return(assetcore.Blob{
			Asset:       assetcore.Asset{ID: id, Kind: assetcore.KindTexture, Source: testTextureProviderName, Title: "Brick Wall"},
			Content:     []byte("fake-zip-bytes"),
			Filename:    "brick-wall_4K-PNG.zip",
			ContentType: "application/zip",
		}, nil)

	res, err := s.handleGetTexture(t.Context(), newRequest(map[string]any{
		"id":         id,
		"resolution": "4K",
		"format":     "PNG",
	}))
	if err != nil {
		t.Fatalf("handleGetTexture: unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleGetTexture: result is an error: %+v", res.Content)
	}
}

func TestHandleGetTextureMissingID(t *testing.T) {
	s, _, _ := newPhotoTextureServer(t)

	res, err := s.handleGetTexture(t.Context(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handleGetTexture: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetTexture: expected IsError=true for missing id")
	}
}

func TestHandleGetTextureNotFound(t *testing.T) {
	s, _, textureProv := newPhotoTextureServer(t)

	textureProv.EXPECT().
		Fetch(mock.Anything, "missing", assetcore.TextureFetchOpts{Resolution: defaultTextureResolution, Format: defaultTextureFormat}).
		Return(assetcore.Blob{}, assetcore.ErrNotFound)

	res, err := s.handleGetTexture(t.Context(), newRequest(map[string]any{
		"id": testTextureProviderName + ":missing",
	}))
	if err != nil {
		t.Fatalf("handleGetTexture: unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("handleGetTexture: expected IsError=true for unknown texture")
	}
}

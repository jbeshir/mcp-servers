package gameart

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/assetsdb/format"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/stretchr/testify/require"
)

func fixtureDB(t *testing.T) *format.DB {
	t.Helper()
	dir := t.TempDir()
	pack := format.Source{Name: "tiny-pack", Title: "Tiny Pack", Path: "sources/tiny-pack.zip", Origin: "https://example.test/tiny", Tags: []string{"pixel"}, Licenses: []format.License{{Name: "CC0-1.0", Title: "CC Zero", Path: "https://creativecommons.org/publicdomain/zero/1.0/"}}}
	items := []format.Item{
		{Name: "hero", Title: "Hero", ID: "assetsdb:tiny-pack/sprites/sheet.png#hero", Source: pack.Name, Kind: format.KindSprite2D, Path: "sprites/sheet.png", MediaType: "image/png", Tokens: []string{"knight"}, Region: &format.Region{X: 1, Y: 2, Width: 8, Height: 9}},
		{Name: "tree", Title: "Tree", ID: "assetsdb:tiny-pack/models/tree.glb", Source: pack.Name, Kind: format.KindModel3D, Path: "models/tree.glb", MediaType: "model/gltf-binary"},
		{Name: "bell", Title: "Bell", ID: "assetsdb:tiny-pack/audio/bell.ogg", Source: pack.Name, Kind: format.KindAudio, Path: "audio/bell.ogg", MediaType: "audio/ogg"},
		{Name: "pixel", Title: "Pixel Font", ID: "assetsdb:tiny-pack/fonts/pixel.ttf", Source: pack.Name, Kind: format.KindFont, Path: "fonts/pixel.ttf", MediaType: "font/ttf"},
	}
	require.NoError(t, format.Write(dir, &format.DataPackage{Name: "fixture", Title: "Fixture", Version: "1", Created: "2026-07-18T00:00:00Z", SchemaVersion: 1, Sources: []format.Source{pack}, Resources: items}))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sources"), 0o750))
	// #nosec G304 -- the path is a fixed test fixture beneath t.TempDir.
	f, err := os.Create(filepath.Join(dir, pack.Path))
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for _, item := range items {
		w, e := zw.Create(item.Path)
		require.NoError(t, e)
		_, e = w.Write([]byte("bytes:" + item.Name))
		require.NoError(t, e)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	db, err := format.Read(dir)
	require.NoError(t, err)
	return db
}

func TestProvidersSearchAndFetchRealZIP(t *testing.T) {
	db := fixtureDB(t)
	sprites := NewSprites(db)
	res, err := sprites.Search(context.Background(), assetcore.SearchOpts{Query: "hero knight", Providers: assetcore.Filter{Only: []string{"assetsdb"}}, Sources: assetcore.Filter{Only: []string{"tiny-pack"}}})
	require.NoError(t, err)
	require.Len(t, res.Assets, 1)
	a := res.Assets[0]
	require.Equal(t, "assetsdb:tiny-pack/sprites/sheet.png#hero", a.ID)
	require.Equal(t, "CC0-1.0", a.License.SPDX)
	require.Equal(t, "Tiny Pack", a.Meta[assetcore.MetaPackTitle])
	require.Equal(t, "8", a.Meta["region_width"])
	b, err := sprites.Fetch(context.Background(), "tiny-pack/sprites/sheet.png#hero", assetcore.SpriteFetchOpts{})
	require.NoError(t, err)
	require.Equal(t, []byte("bytes:hero"), b.Content)
	require.Equal(t, "sheet.png", b.Filename)
	require.Equal(t, "image/png", b.ContentType)
	_, err = sprites.Fetch(context.Background(), "missing", assetcore.SpriteFetchOpts{})
	require.ErrorIs(t, err, assetcore.ErrNotFound)
	require.NotEmpty(t, NewModels(db).Sources())
	require.NotEmpty(t, NewAudio(db).Sources())
	require.NotEmpty(t, NewFonts(db).Sources())
	_, err = sprites.Search(context.Background(), assetcore.SearchOpts{Providers: assetcore.Filter{Except: []string{"assetsdb"}}})
	require.NotErrorIs(t, err, assetcore.ErrNotFound)
}

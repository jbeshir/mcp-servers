package assetsdb

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	upstream "github.com/jbeshir/assetsdb"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/stretchr/testify/require"
)

func fixtureDB(t *testing.T) *upstream.DB {
	t.Helper()
	dir := t.TempDir()
	pack := upstream.Source{Name: "tiny-pack", Title: "Tiny Pack", Path: "sources/tiny-pack.zip", Origin: "https://example.test/tiny", Tags: []string{"pixel"}, Licenses: []upstream.License{{Name: "CC0-1.0", Title: "CC Zero", Path: "https://creativecommons.org/publicdomain/zero/1.0/"}}}
	items := []upstream.Item{
		{Name: "hero", Title: "Hero", ID: "assetsdb:tiny-pack/sprites/sheet.png#hero", Source: pack.Name, Kind: upstream.KindSprite2D, Path: "sprites/sheet.png", MediaType: "image/png", Tokens: []string{"knight"}, Region: &upstream.Region{X: 1, Y: 2, Width: 8, Height: 9}},
		{Name: "shield", Title: "Shield", ID: "assetsdb:tiny-pack/sprites/shield.png", Source: pack.Name, Kind: upstream.KindSprite2D, Path: "sprites/shield.png", MediaType: "image/png", Tokens: []string{"knight"}},
		{Name: "tree", Title: "Tree", ID: "assetsdb:tiny-pack/models/tree.glb", Source: pack.Name, Kind: upstream.KindModel3D, Path: "models/tree.glb", MediaType: "model/gltf-binary"},
		{Name: "bell", Title: "Bell", ID: "assetsdb:tiny-pack/audio/bell.ogg", Source: pack.Name, Kind: upstream.KindAudio, Path: "audio/bell.ogg", MediaType: "audio/ogg"},
		{Name: "pixel", Title: "Pixel Font", ID: "assetsdb:tiny-pack/fonts/pixel.ttf", Source: pack.Name, Kind: upstream.KindFont, Path: "fonts/pixel.ttf", MediaType: "font/ttf"},
	}
	writeDataPackage(t, dir, &upstream.DataPackage{Name: "fixture", Title: "Fixture", Version: "1", Created: "2026-07-18T00:00:00Z", SchemaVersion: 1, Sources: []upstream.Source{pack}, Resources: items})
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
	db, err := upstream.Read(dir)
	require.NoError(t, err)
	return db
}

func TestSearchPaginatesDeterministically(t *testing.T) {
	sprites := New(fixtureDB(t)).Sprites()

	first, err := sprites.Search(t.Context(), assetcore.SearchOpts{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first.Assets, 1)
	require.Equal(t, "1", first.NextCursor)

	second, err := sprites.Search(t.Context(), assetcore.SearchOpts{Limit: 1, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Assets, 1)
	require.Empty(t, second.NextCursor)
	require.NotEqual(t, first.Assets[0].ID, second.Assets[0].ID)

	_, err = sprites.Search(t.Context(), assetcore.SearchOpts{Cursor: "invalid"})
	require.Error(t, err)
}

func writeDataPackage(t *testing.T, dir string, dataPackage *upstream.DataPackage) {
	t.Helper()

	// #nosec G304 -- the path is a fixed test fixture beneath t.TempDir.
	file, err := os.Create(filepath.Join(dir, "datapackage.json"))
	require.NoError(t, err)
	require.NoError(t, upstream.Encode(file, dataPackage))
	require.NoError(t, file.Close())
}

func TestProvidersSearchAndFetchRealZIP(t *testing.T) {
	db := fixtureDB(t)
	catalog := New(db)
	sprites := catalog.Sprites()
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
	require.NotEmpty(t, catalog.Models().Sources())
	require.NotEmpty(t, catalog.Audio().Sources())
	require.NotEmpty(t, catalog.Fonts().Sources())
	_, err = sprites.Search(context.Background(), assetcore.SearchOpts{Providers: assetcore.Filter{Except: []string{"assetsdb"}}})
	require.NotErrorIs(t, err, assetcore.ErrNotFound)
}

func TestPackDiscoveryAndOpenUseDatabaseSources(t *testing.T) {
	catalog := New(fixtureDB(t))

	packs := catalog.Packs()
	require.Len(t, packs, 1)
	require.Equal(t, "tiny-pack", packs[0].ID)
	require.Equal(t, 5, packs[0].Count)
	require.Equal(t, 2, packs[0].Kinds[assetcore.KindSprite])
	require.Equal(t, 1, packs[0].Kinds[assetcore.KindModel])
	require.Equal(t, 1, packs[0].Kinds[assetcore.KindAudio])
	require.Equal(t, 1, packs[0].Kinds[assetcore.KindFont])

	reader, pack, err := catalog.OpenPack("tiny-pack")
	require.NoError(t, err)
	require.Equal(t, packs[0], pack)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.True(t, bytes.HasPrefix(content, []byte("PK")))

	_, _, err = catalog.OpenPack("../tiny-pack")
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

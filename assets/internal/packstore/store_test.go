package packstore

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/assetsdb/format"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/stretchr/testify/require"
)

func TestStoreOpensOnlyIndexedOriginalZIP(t *testing.T) {
	dir := t.TempDir()
	src := format.Source{Name: "pack", Title: "Pack", Path: "sources/pack.zip", Licenses: []format.License{{Name: "CC0-1.0"}}}
	item := format.Item{Name: "a", Title: "A", ID: "assetsdb:pack/a.png", Source: "pack", Kind: format.KindSprite2D, Path: "a.png"}
	require.NoError(t, format.Write(dir, &format.DataPackage{Name: "fixture", Title: "Fixture", Version: "1", Created: "now", SchemaVersion: 1, Sources: []format.Source{src}, Resources: []format.Item{item}}))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sources"), 0o750))
	// #nosec G304 -- the path is a fixed test fixture beneath t.TempDir.
	f, err := os.Create(filepath.Join(dir, src.Path))
	require.NoError(t, err)
	z := zip.NewWriter(f)
	w, err := z.Create("a.png")
	require.NoError(t, err)
	_, err = w.Write([]byte("png"))
	require.NoError(t, err)
	require.NoError(t, z.Close())
	require.NoError(t, f.Close())
	db, err := format.Read(dir)
	require.NoError(t, err)
	s := New(dir, db)
	require.Len(t, s.List(), 1)
	r, p, err := s.Open("pack")
	require.NoError(t, err)
	require.Equal(t, 1, p.Count)
	require.Equal(t, 1, p.Kinds[assetcore.KindSprite])
	require.NoError(t, r.Close())
	_, _, err = s.Open("../pack")
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

func TestStoreRejectsTraversalPathFromIndex(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.zip")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	rel, err := filepath.Rel(dir, outside)
	require.NoError(t, err)
	src := format.Source{Name: "evil", Title: "Evil", Path: filepath.ToSlash(rel)}
	item := format.Item{Name: "a", ID: "assetsdb:evil/a.png", Source: "evil", Kind: format.KindSprite2D, Path: "a.png"}
	require.NoError(t, format.Write(dir, &format.DataPackage{Name: "fixture", Title: "Fixture", Version: "1", Created: "now", SchemaVersion: 1, Sources: []format.Source{src}, Resources: []format.Item{item}}))
	db, err := format.Read(dir)
	require.NoError(t, err)
	_, _, err = New(dir, db).Open("evil")
	require.ErrorIs(t, err, assetcore.ErrNotFound)
}

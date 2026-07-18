// Package packstore provides traversal-safe access to original assetsdb pack ZIPs.
package packstore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jbeshir/assetsdb/format"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

type Pack struct {
	ID, Title string
	Tags      []string
	Count     int
	License   assetcore.License
	Kinds     map[assetcore.Kind]int
}
type Store interface {
	List() []Pack
	Open(string) (io.ReadCloser, Pack, error)
}
type FS struct {
	root  string
	packs map[string]Pack
	paths map[string]string
}

func New(root string, db *format.DB) *FS {
	f := &FS{root: filepath.Clean(root), packs: map[string]Pack{}, paths: map[string]string{}}
	for _, item := range db.Search("") {
		src, ok := db.SourceByID(item.Source)
		if !ok {
			continue
		}
		p := f.packs[item.Source]
		p.ID = item.Source
		p.Title = src.Title
		p.Tags = append([]string(nil), src.Tags...)
		p.Count++
		if p.Kinds == nil {
			p.Kinds = map[assetcore.Kind]int{}
		}
		if k, ok := coreKind(item.Kind); ok {
			p.Kinds[k]++
		}
		if len(src.Licenses) > 0 {
			l := src.Licenses[0]
			p.License = assetcore.License{SPDX: l.Name, Name: l.Title, URL: l.Path}
		}
		f.packs[item.Source] = p
		candidate := filepath.Join(f.root, filepath.FromSlash(src.Path))
		if within(f.root, candidate) {
			f.paths[item.Source] = candidate
		}
	}
	return f
}
func coreKind(k format.Kind) (assetcore.Kind, bool) {
	switch k {
	case format.KindModel3D:
		return assetcore.KindModel, true
	case format.KindAudio:
		return assetcore.KindAudio, true
	case format.KindFont:
		return assetcore.KindFont, true
	case format.KindSprite2D:
		return assetcore.KindSprite, true
	}
	return "", false
}
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (f *FS) List() []Pack {
	out := make([]Pack, 0, len(f.packs))
	for _, p := range f.packs {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (f *FS) Open(id string) (io.ReadCloser, Pack, error) {
	p, ok := f.packs[id]
	if !ok {
		return nil, Pack{}, fmt.Errorf("%w: pack %q", assetcore.ErrNotFound, id)
	}
	name, ok := f.paths[id]
	if !ok {
		return nil, Pack{}, fmt.Errorf("%w: unsafe pack path", assetcore.ErrNotFound)
	}
	r, err := os.Open(name)
	if os.IsNotExist(err) {
		err = fmt.Errorf("%w: pack file", assetcore.ErrNotFound)
	}
	return r, p, err
}

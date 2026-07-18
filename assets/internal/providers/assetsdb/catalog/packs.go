package catalog

import (
	"errors"
	"fmt"
	"io"

	"github.com/jbeshir/assetsdb"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

// Packs returns the upstream sources as downloadable asset packs.
func (c *Catalog) Packs() []assetcore.Pack {
	sources := c.db.Sources()
	packs := make([]assetcore.Pack, 0, len(sources))
	for _, source := range sources {
		packs = append(packs, c.pack(source))
	}

	return packs
}

// OpenPack opens the raw archive registered for id through the upstream assetsdb API.
func (c *Catalog) OpenPack(id string) (io.ReadCloser, assetcore.Pack, error) {
	source, ok := c.db.SourceByID(id)
	if !ok {
		return nil, assetcore.Pack{}, fmt.Errorf("%w: pack %q", assetcore.ErrNotFound, id)
	}

	reader, err := c.db.OpenSource(id)
	if errors.Is(err, assetsdb.ErrNotFound) {
		return nil, assetcore.Pack{}, fmt.Errorf("%w: pack %q", assetcore.ErrNotFound, id)
	}
	if err != nil {
		return nil, assetcore.Pack{}, fmt.Errorf("open assetsdb pack %q: %w", id, err)
	}

	return reader, c.pack(source), nil
}

func (c *Catalog) pack(source assetsdb.Source) assetcore.Pack {
	items := c.db.ItemsForSource(source.Name)
	pack := assetcore.Pack{
		ID:      source.Name,
		Title:   source.Title,
		Tags:    append([]string(nil), source.Tags...),
		Count:   len(items),
		License: sourceLicense(source),
		Kinds:   make(map[assetcore.Kind]int),
	}

	for _, item := range items {
		if kind, ok := coreKind(item.Kind); ok {
			pack.Kinds[kind]++
		}
	}

	return pack
}

func coreKind(kind assetsdb.Kind) (assetcore.Kind, bool) {
	switch kind {
	case assetsdb.KindModel3D:
		return assetcore.KindModel, true
	case assetsdb.KindAudio:
		return assetcore.KindAudio, true
	case assetsdb.KindFont:
		return assetcore.KindFont, true
	case assetsdb.KindSprite2D:
		return assetcore.KindSprite, true
	default:
		return "", false
	}
}

var _ assetcore.PackStore = (*Catalog)(nil)

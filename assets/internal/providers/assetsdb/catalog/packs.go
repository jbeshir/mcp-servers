package catalog

import (
	"errors"
	"fmt"
	"io"
	"strconv"

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

// ListPackAssets enumerates a pack's catalogued assets without requiring a search query.
func (c *Catalog) ListPackAssets(
	packID string,
	kind assetcore.Kind,
	limit int,
	cursor string,
) (assetcore.SearchResult, error) {
	if _, ok := c.db.SourceByID(packID); !ok {
		return assetcore.SearchResult{}, fmt.Errorf("%w: pack %q", assetcore.ErrNotFound, packID)
	}

	offset := 0
	if cursor != "" {
		var err error
		offset, err = strconv.Atoi(cursor)
		if err != nil || offset < 0 {
			return assetcore.SearchResult{}, fmt.Errorf("assetsdb: invalid cursor %q", cursor)
		}
	}

	items := c.db.ItemsForSource(packID)
	assets := make([]assetcore.Asset, 0, len(items))
	for _, item := range items {
		coreKind, ok := coreKind(item.Kind)
		if !ok || kind != "" && coreKind != kind {
			continue
		}
		assets = append(assets, c.provider(item.Kind, coreKind).asset(item))
	}

	if offset >= len(assets) {
		return assetcore.SearchResult{}, nil
	}
	end := min(offset+assetcore.ClampLimit(limit), len(assets))
	result := assetcore.SearchResult{Assets: assets[offset:end]}
	if end < len(assets) {
		result.NextCursor = strconv.Itoa(end)
	}
	return result, nil
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

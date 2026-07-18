// Package assetsdb adapts an assetsdb database into the server's local asset providers.
package assetsdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	upstream "github.com/jbeshir/assetsdb"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

const providerName = "assetsdb"

// Catalog owns the shared assetsdb adapter state used by the kind-specific providers and pack API.
type Catalog struct {
	db        *upstream.DB
	itemsByID map[string]upstream.Item
}

type provider struct {
	catalog  *Catalog
	dbKind   upstream.Kind
	coreKind assetcore.Kind
}

type Models struct{ *provider }
type Audio struct{ *provider }
type Fonts struct{ *provider }
type Sprites struct{ *provider }

// New creates one catalog over db. Its kind-specific provider views share the same immutable index.
func New(db *upstream.DB) *Catalog {
	itemsByID := make(map[string]upstream.Item)
	for _, source := range db.Sources() {
		for _, item := range db.ItemsForSource(source.Name) {
			localID, ok := localID(item.ID)
			if ok {
				itemsByID[localID] = item
			}
		}
	}

	return &Catalog{db: db, itemsByID: itemsByID}
}

func (c *Catalog) Models() *Models {
	return &Models{c.provider(upstream.KindModel3D, assetcore.KindModel)}
}

func (c *Catalog) Audio() *Audio {
	return &Audio{c.provider(upstream.KindAudio, assetcore.KindAudio)}
}

func (c *Catalog) Fonts() *Fonts {
	return &Fonts{c.provider(upstream.KindFont, assetcore.KindFont)}
}

func (c *Catalog) Sprites() *Sprites {
	return &Sprites{c.provider(upstream.KindSprite2D, assetcore.KindSprite)}
}

func (c *Catalog) provider(dbKind upstream.Kind, coreKind assetcore.Kind) *provider {
	return &provider{catalog: c, dbKind: dbKind, coreKind: coreKind}
}

func (p *provider) Name() string         { return providerName }
func (p *provider) Kind() assetcore.Kind { return p.coreKind }

func (p *provider) Search(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if !opts.Providers.Allows(providerName) {
		return assetcore.SearchResult{}, nil
	}

	offset, err := searchOffset(opts.Cursor)
	if err != nil {
		return assetcore.SearchResult{}, err
	}

	limit := assetcore.ClampLimit(opts.Limit)
	matches := make([]upstream.Item, 0)
	for _, item := range p.catalog.db.Search(opts.Query) {
		if item.Kind != p.dbKind || !opts.Sources.Allows(item.Source) {
			continue
		}

		matches = append(matches, item)
	}

	if offset >= len(matches) {
		return assetcore.SearchResult{}, nil
	}

	end := min(offset+limit, len(matches))
	assets := make([]assetcore.Asset, 0, end-offset)
	for _, item := range matches[offset:end] {
		assets = append(assets, p.asset(item))
	}

	nextCursor := ""
	if end < len(matches) {
		nextCursor = strconv.Itoa(end)
	}

	return assetcore.SearchResult{Assets: assets, NextCursor: nextCursor}, nil
}

func searchOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("assetsdb: invalid cursor %q", cursor)
	}

	return offset, nil
}

func (p *provider) asset(item upstream.Item) assetcore.Asset {
	id, _ := localID(item.ID)
	license := p.catalog.db.LicenseFor(item)
	title := item.Title
	if title == "" {
		title = item.Name
	}

	asset := assetcore.Asset{
		Source:  item.Source,
		ID:      assetcore.AssetID(providerName, id),
		Kind:    p.coreKind,
		Title:   title,
		Tags:    append([]string(nil), item.Tokens...),
		License: coreLicense(license),
		Meta: map[string]string{
			assetcore.MetaPack: item.Source,
			"kind":             string(p.coreKind),
		},
	}

	if source, ok := p.catalog.db.SourceByID(item.Source); ok {
		asset.LandingURL = source.Origin
		asset.Meta[assetcore.MetaPackTitle] = source.Title
	}
	addRegionMetadata(asset.Meta, item.Region)

	return asset
}

func (p *provider) fetch(id string) (assetcore.Blob, error) {
	item, ok := p.catalog.itemsByID[id]
	if !ok || item.Kind != p.dbKind {
		return assetcore.Blob{}, notFound(id)
	}

	reader, err := p.catalog.db.Open(item)
	if err != nil {
		if errors.Is(err, upstream.ErrNotFound) {
			return assetcore.Blob{}, notFound(id)
		}
		return assetcore.Blob{}, fmt.Errorf("open assetsdb item %s: %w", id, err)
	}

	content, err := readAndClose(reader)
	if err != nil {
		return assetcore.Blob{}, fmt.Errorf("read assetsdb item %s: %w", id, err)
	}

	return assetcore.Blob{
		Asset:       p.asset(item),
		Content:     content,
		Filename:    path.Base(item.Path),
		ContentType: item.MediaType,
	}, nil
}

func (p *provider) Sources() []assetcore.Source {
	sources := make([]assetcore.Source, 0)
	for _, source := range p.catalog.db.Sources() {
		count := countKind(p.catalog.db.ItemsForSource(source.Name), p.dbKind)
		if count == 0 {
			continue
		}

		sources = append(sources, assetcore.Source{
			Name:    source.Name,
			Count:   count,
			License: sourceLicense(source),
			Meta: map[string]string{
				assetcore.MetaPackTitle: source.Title,
				"tags":                  strings.Join(source.Tags, ","),
				"origin":                source.Origin,
			},
		})
	}

	return sources
}

func localID(id string) (string, bool) {
	local := strings.TrimPrefix(id, providerName+":")
	return local, local != "" && local != id
}

func coreLicense(license upstream.License) assetcore.License {
	return assetcore.License{SPDX: license.Name, Name: license.Title, URL: license.Path}
}

func sourceLicense(source upstream.Source) assetcore.License {
	if len(source.Licenses) == 0 {
		return assetcore.License{}
	}

	return coreLicense(source.Licenses[0])
}

func countKind(items []upstream.Item, kind upstream.Kind) int {
	count := 0
	for _, item := range items {
		if item.Kind == kind {
			count++
		}
	}

	return count
}

func addRegionMetadata(metadata map[string]string, region *upstream.Region) {
	if region == nil {
		return
	}

	metadata["region_x"] = strconv.Itoa(region.X)
	metadata["region_y"] = strconv.Itoa(region.Y)
	metadata["region_width"] = strconv.Itoa(region.Width)
	metadata["region_height"] = strconv.Itoa(region.Height)
}

func readAndClose(reader io.ReadCloser) ([]byte, error) {
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}

	return content, closeErr
}

func notFound(id string) error {
	return fmt.Errorf("%w: assetsdb:%s", assetcore.ErrNotFound, id)
}

func (p *Models) Fetch(_ context.Context, id string, _ assetcore.ModelFetchOpts) (assetcore.Blob, error) {
	return p.fetch(id)
}

func (p *Audio) Fetch(_ context.Context, id string, _ assetcore.AudioFetchOpts) (assetcore.Blob, error) {
	return p.fetch(id)
}

func (p *Fonts) Fetch(_ context.Context, id string, _ assetcore.FontFetchOpts) (assetcore.Blob, error) {
	return p.fetch(id)
}

func (p *Sprites) Fetch(_ context.Context, id string, _ assetcore.SpriteFetchOpts) (assetcore.Blob, error) {
	return p.fetch(id)
}

var _ assetcore.ModelProvider = (*Models)(nil)
var _ assetcore.AudioProvider = (*Audio)(nil)
var _ assetcore.FontProvider = (*Fonts)(nil)
var _ assetcore.SpriteProvider = (*Sprites)(nil)

// Package gameart adapts an assetsdb database into the server's four local game-art providers.
package gameart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jbeshir/assetsdb/format"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

const providerName = "assetsdb"

type provider struct {
	db        *format.DB
	dbKind    format.Kind
	coreKind  assetcore.Kind
	itemsByID map[string]format.Item
}

type Models struct{ *provider }
type Audio struct{ *provider }
type Fonts struct{ *provider }
type Sprites struct{ *provider }

func newProvider(db *format.DB, dk format.Kind, ck assetcore.Kind) *provider {
	p := &provider{db: db, dbKind: dk, coreKind: ck, itemsByID: make(map[string]format.Item)}
	for _, item := range db.Search("") {
		if item.Kind != dk || !strings.HasPrefix(item.ID, "assetsdb:") {
			continue
		}
		local := strings.TrimPrefix(item.ID, "assetsdb:")
		if local != "" {
			p.itemsByID[local] = item
		}
	}
	return p
}

func NewModels(db *format.DB) *Models {
	return &Models{newProvider(db, format.KindModel3D, assetcore.KindModel)}
}
func NewAudio(db *format.DB) *Audio {
	return &Audio{newProvider(db, format.KindAudio, assetcore.KindAudio)}
}
func NewFonts(db *format.DB) *Fonts {
	return &Fonts{newProvider(db, format.KindFont, assetcore.KindFont)}
}
func NewSprites(db *format.DB) *Sprites {
	return &Sprites{newProvider(db, format.KindSprite2D, assetcore.KindSprite)}
}

func (p *provider) Name() string         { return providerName }
func (p *provider) Kind() assetcore.Kind { return p.coreKind }

func (p *provider) Search(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	if !opts.Providers.Allows(providerName) {
		return assetcore.SearchResult{}, nil
	}
	limit := assetcore.ClampLimit(opts.Limit)
	out := make([]assetcore.Asset, 0, limit)
	for _, item := range p.db.Search(opts.Query) {
		if item.Kind != p.dbKind || !opts.Sources.Allows(item.Source) {
			continue
		}
		out = append(out, p.asset(item))
		if len(out) == limit {
			break
		}
	}
	return assetcore.SearchResult{Assets: out}, nil
}

func (p *provider) asset(item format.Item) assetcore.Asset {
	local := strings.TrimPrefix(item.ID, "assetsdb:")
	lic := p.db.LicenseFor(item)
	title := item.Title
	if title == "" {
		title = item.Name
	}
	a := assetcore.Asset{Source: item.Source, ID: assetcore.AssetID(providerName, local), Kind: p.coreKind,
		Title: title, Tags: append([]string(nil), item.Tokens...),
		License: assetcore.License{SPDX: lic.Name, Name: lic.Title, URL: lic.Path},
		Meta:    map[string]string{assetcore.MetaPack: item.Source, "kind": string(p.coreKind)}}
	if src, ok := p.db.SourceByID(item.Source); ok {
		a.LandingURL = src.Origin
		a.Meta[assetcore.MetaPackTitle] = src.Title
	}
	if item.Region != nil {
		a.Meta["region_x"] = strconv.Itoa(item.Region.X)
		a.Meta["region_y"] = strconv.Itoa(item.Region.Y)
		a.Meta["region_width"] = strconv.Itoa(item.Region.Width)
		a.Meta["region_height"] = strconv.Itoa(item.Region.Height)
	}
	return a
}

func (p *provider) fetch(id string) (assetcore.Blob, error) {
	item, ok := p.itemsByID[id]
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("%w: assetsdb:%s", assetcore.ErrNotFound, id)
	}
	r, err := p.db.Open(item)
	if err != nil {
		if errors.Is(err, format.ErrNotFound) {
			return assetcore.Blob{}, fmt.Errorf("%w: assetsdb:%s", assetcore.ErrNotFound, id)
		}
		return assetcore.Blob{}, fmt.Errorf("open assetsdb item %s: %w", id, err)
	}
	b, readErr := io.ReadAll(r)
	closeErr := r.Close()
	if readErr != nil {
		return assetcore.Blob{}, fmt.Errorf("read assetsdb item %s: %w", id, readErr)
	}
	if closeErr != nil {
		return assetcore.Blob{}, fmt.Errorf("close assetsdb item %s: %w", id, closeErr)
	}
	return assetcore.Blob{Asset: p.asset(item), Content: b, Filename: path.Base(item.Path), ContentType: item.MediaType}, nil
}

func (p *provider) Sources() []assetcore.Source {
	counts := map[string]int{}
	for _, item := range p.itemsByID {
		counts[item.Source]++
	}
	out := make([]assetcore.Source, 0, len(counts))
	for id, count := range counts {
		s := assetcore.Source{Name: id, Count: count, Meta: map[string]string{}}
		if src, ok := p.db.SourceByID(id); ok {
			if len(src.Licenses) > 0 {
				l := src.Licenses[0]
				s.License = assetcore.License{SPDX: l.Name, Name: l.Title, URL: l.Path}
			}
			s.Meta[assetcore.MetaPackTitle] = src.Title
			s.Meta["tags"] = strings.Join(src.Tags, ",")
			s.Meta["origin"] = src.Origin
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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

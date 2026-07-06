// Package embedded adapts the internal/icons kind package to the assetcore.IconProvider interface.
// It is a strangler-fig adapter: it delegates search and render to the existing icons functions and
// looks up licensing via the catalog, mapping the results onto assetcore value types. The embedded
// icon data and rendering logic are not moved or rewritten here.
package embedded

import (
	"context"
	"errors"
	"strconv"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
	"github.com/jbeshir/mcp-servers/assets/internal/icons"
)

// providerName is the stable registry key for the embedded icon provider.
const providerName = "embedded-icons"

// Provider satisfies assetcore.IconProvider.
var _ assetcore.IconProvider = (*Provider)(nil)

// Provider serves the vendored Iconify icon sets as an assetcore.IconProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded icon provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves icons.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIcon }

// Search delegates to icons.Search and maps each hit onto an assetcore.Asset.
func (p *Provider) Search(_ context.Context, q assetcore.IconQuery) (assetcore.Page, error) {
	results := icons.Search(q.Query, q.Set, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.Set, m.Name))
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch renders the icon identified by a (Source=set, Title=name) with the colour/size hints carried
// in a.Ref, delegating to icons.Render. An unknown set or icon is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	set, name := a.Source, a.Title
	color := a.Ref[assetcore.RefColor]
	size, _ := strconv.Atoi(a.Ref[assetcore.RefSize])

	data, err := icons.Render(set, name, color, size)
	if err != nil {
		if errors.Is(err, icons.ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	asset := p.asset(set, name)
	asset.Ref = a.Ref

	return assetcore.Blob{
		Asset:       asset,
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}, nil
}

// asset builds the assetcore.Asset for an icon, resolving its license from the catalog.
func (p *Provider) asset(set, name string) assetcore.Asset {
	license, attribution, _ := p.catalog.IconLicense(set)

	return assetcore.Asset{
		Provider: providerName,
		Source:   set,
		ID:       set + "/" + name,
		Kind:     assetcore.KindIcon,
		Title:    name,
		License:  assetcore.License{SPDX: license, Attribution: attribution},
	}
}

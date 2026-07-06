// Package embedded adapts the internal/illustrations kind package to the
// assetcore.IllustrationProvider interface. It is a strangler-fig adapter: it delegates search and
// retrieval to the existing illustrations functions and looks up licensing via the catalog, without
// moving or rewriting the embedded data.
package embedded

import (
	"context"
	"errors"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
	"github.com/jbeshir/mcp-servers/assets/internal/illustrations"
)

// providerName is the stable registry key for the embedded illustration provider.
const providerName = "embedded-illustrations"

// Provider satisfies assetcore.IllustrationProvider.
var _ assetcore.IllustrationProvider = (*Provider)(nil)

// Provider serves the vendored SVG illustration collections as an assetcore.IllustrationProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded illustration provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves illustrations.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIllustration }

// Search delegates to illustrations.Search and maps each hit onto an assetcore.Asset.
func (p *Provider) Search(_ context.Context, q assetcore.IllustrationQuery) (assetcore.Page, error) {
	results := illustrations.Search(q.Query, q.Collection, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.Collection, m.Name))
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch returns the SVG identified by a (Source=collection, Title=name) via illustrations.Get. An
// unknown collection or name is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	collection, name := a.Source, a.Title

	data, err := illustrations.Get(collection, name)
	if err != nil {
		if errors.Is(err, illustrations.ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	return assetcore.Blob{
		Asset:       p.asset(collection, name),
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}, nil
}

// asset builds the assetcore.Asset for an illustration, resolving its license from the catalog.
func (p *Provider) asset(collection, name string) assetcore.Asset {
	license, attribution, _ := p.catalog.IllustrationLicense(collection)

	return assetcore.Asset{
		Provider: providerName,
		Source:   collection,
		ID:       collection + "/" + name,
		Kind:     assetcore.KindIllustration,
		Title:    name,
		License:  assetcore.License{SPDX: license, Attribution: attribution},
	}
}

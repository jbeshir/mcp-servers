// Package embedded adapts the internal/fonts kind package to the assetcore.FontProvider interface,
// additionally implementing assetcore.FontFaceRenderer for @font-face CSS. It is a strangler-fig
// adapter: it delegates search, retrieval, and CSS rendering to the existing fonts functions and
// looks up licensing via the catalog, without moving or rewriting the embedded data.
package embedded

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
	"github.com/jbeshir/mcp-servers/assets/internal/fonts"
)

// providerName is the stable registry key for the embedded font provider.
const providerName = "embedded-fonts"

// woff2ContentType is the MIME type of the vendored font files.
const woff2ContentType = "font/woff2"

// Provider satisfies assetcore.FontProvider and the optional assetcore.FontFaceRenderer capability.
var (
	_ assetcore.FontProvider     = (*Provider)(nil)
	_ assetcore.FontFaceRenderer = (*Provider)(nil)
)

// Provider serves the vendored OFL font families as an assetcore.FontProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded font provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves fonts.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindFont }

// Search delegates to fonts.Search and maps each family onto an assetcore.Asset, carrying the
// family's category and available weights as Ref hints for search-result display.
func (p *Provider) Search(_ context.Context, q assetcore.FontQuery) (assetcore.Page, error) {
	results := fonts.Search(q.Query, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		license, attribution, _ := p.catalog.FontLicense(m.Family)

		weights := make([]string, 0, len(m.Weights))
		for _, w := range m.Weights {
			weights = append(weights, strconv.Itoa(w))
		}

		assets = append(assets, assetcore.Asset{
			Provider: providerName,
			Source:   m.Slug,
			ID:       m.Slug,
			Kind:     assetcore.KindFont,
			Title:    m.Family,
			License:  assetcore.License{SPDX: license, Attribution: attribution},
			Ref: map[string]string{
				assetcore.RefCategory: m.Category,
				assetcore.RefWeights:  strings.Join(weights, ","),
			},
		})
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch returns the woff2 identified by a (Source=family) with the weight/style hints carried in
// a.Ref, delegating to fonts.Get. The Blob's Filename is the internal font filename (referenced by
// @font-face CSS), not the caller's output filename. An unknown family/weight/style is reported as
// assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	family := a.Source
	weight, _ := strconv.Atoi(a.Ref[assetcore.RefWeight])
	style := a.Ref[assetcore.RefStyle]

	font, err := fonts.Get(family, weight, style)
	if err != nil {
		if errors.Is(err, fonts.ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	license, attribution, _ := p.catalog.FontLicense(family)

	asset := a
	asset.Provider = providerName
	asset.Kind = assetcore.KindFont
	asset.License = assetcore.License{SPDX: license, Attribution: attribution}

	return assetcore.Blob{
		Asset:       asset,
		Content:     font.Data,
		Filename:    font.Filename,
		ContentType: woff2ContentType,
	}, nil
}

// RenderFontFace renders an @font-face CSS snippet for a font Blob produced by Fetch, delegating to
// fonts.FontFace. It reconstructs the fonts.Font from the Blob's internal filename and the weight/
// style hints on the Blob's Asset. It satisfies assetcore.FontFaceRenderer.
func (p *Provider) RenderFontFace(familyDisplay string, b assetcore.Blob) string {
	weight, _ := strconv.Atoi(b.Asset.Ref[assetcore.RefWeight])
	style := b.Asset.Ref[assetcore.RefStyle]

	return fonts.FontFace(familyDisplay, fonts.Font{
		Filename: b.Filename,
		Weight:   weight,
		Style:    style,
	})
}

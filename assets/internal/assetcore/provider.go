package assetcore

import "context"

// Provider is a tiny shared identity marker, not a behaviour interface. Every concrete provider
// exposes a stable registry Name and the single Kind it serves.
type Provider interface {
	Name() string
	Kind() Kind
}

// IconFetchOpts carries the render parameters for an icon fetch that are not part of the icon's
// identity. Zero values mean "native": empty Color leaves the SVG uncoloured, Size 0 keeps the icon's
// native grid.
type IconFetchOpts struct {
	Color string
	Size  int
}

// FontFetchOpts carries the variant selectors for a font fetch. A zero Weight resolves to 400 and an
// empty Style to "normal".
type FontFetchOpts struct {
	Weight int
	Style  string
}

// PhotoFetchOpts carries the (currently empty) fetch parameters for a photo. Photos are fetched by id
// with no render parameters; the typed opts keeps the per-kind Fetch signature uniform.
type PhotoFetchOpts struct{}

// TextureFetchOpts selects which material archive to download. Zero values resolve to 1K/JPG.
type TextureFetchOpts struct {
	Resolution string
	Format     string
}

// IconProvider serves icon assets. Search finds icons; Fetch materializes one from its provider-local
// id (the local half of a composite Asset.ID), rendered per the typed opts.
type IconProvider interface {
	Provider
	Search(ctx context.Context, opts SearchOpts) (SearchResult, error)
	Fetch(ctx context.Context, id string, opts IconFetchOpts) (Blob, error)
}

// IllustrationProvider serves illustration assets. Illustrations have no render parameters, so Fetch
// takes only the provider-local id.
type IllustrationProvider interface {
	Provider
	Search(ctx context.Context, opts SearchOpts) (SearchResult, error)
	Fetch(ctx context.Context, id string) (Blob, error)
}

// FontProvider serves font assets. Fetch takes the provider-local id (a family slug) plus the variant
// selectors in FontFetchOpts.
type FontProvider interface {
	Provider
	Search(ctx context.Context, opts SearchOpts) (SearchResult, error)
	Fetch(ctx context.Context, id string, opts FontFetchOpts) (Blob, error)
}

// PhotoProvider serves photo assets. Photos have no render parameters, so Fetch takes the
// provider-local id plus the (empty) PhotoFetchOpts, kept for signature uniformity with the other
// per-kind Fetch methods.
type PhotoProvider interface {
	Provider
	Search(ctx context.Context, opts SearchOpts) (SearchResult, error)
	Fetch(ctx context.Context, id string, opts PhotoFetchOpts) (Blob, error)
}

// TextureProvider serves texture assets. Fetch takes the provider-local id plus the resolution/format
// selectors in TextureFetchOpts.
type TextureProvider interface {
	Provider
	Search(ctx context.Context, opts SearchOpts) (SearchResult, error)
	Fetch(ctx context.Context, id string, opts TextureFetchOpts) (Blob, error)
}

// FontFaceRenderer is an optional capability a FontProvider may implement to render an @font-face CSS
// snippet for a font Blob it produced. Callers type-assert to discover it, keeping CSS generation out
// of the core FontProvider contract (interface segregation). familyDisplay is the family label to
// embed verbatim in the snippet.
type FontFaceRenderer interface {
	RenderFontFace(familyDisplay string, b Blob) string
}

// SourceLister is an optional capability a provider may implement to enumerate the upstream sources it
// serves, for discovery (list_asset_sources). The embedded providers implement it; a remote aggregator
// that cannot cheaply enumerate its catalogue may omit it.
type SourceLister interface {
	Sources() []Source
}

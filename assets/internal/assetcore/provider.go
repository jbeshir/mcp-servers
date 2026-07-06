package assetcore

import "context"

// Provider is a tiny shared identity marker, not a behaviour interface. Every concrete provider
// exposes a stable registry Name and the single Kind it serves.
type Provider interface {
	Name() string
	Kind() Kind
}

// IconProvider serves icon assets. Search finds icons; Fetch materializes one from an Asset. Render
// parameters that are not part of an icon's identity (colour, size) travel in Asset.Ref, so callers
// that fetch by identity (get_icon has no prior search) simply construct the Asset and set Ref.
type IconProvider interface {
	Provider
	Search(ctx context.Context, q IconQuery) (Page, error)
	Fetch(ctx context.Context, a Asset) (Blob, error)
}

// IllustrationProvider serves illustration assets.
type IllustrationProvider interface {
	Provider
	Search(ctx context.Context, q IllustrationQuery) (Page, error)
	Fetch(ctx context.Context, a Asset) (Blob, error)
}

// FontProvider serves font assets. Weight and style are fetch-time parameters carried in Asset.Ref.
type FontProvider interface {
	Provider
	Search(ctx context.Context, q FontQuery) (Page, error)
	Fetch(ctx context.Context, a Asset) (Blob, error)
}

// FontFaceRenderer is an optional capability a FontProvider may implement to render an @font-face CSS
// snippet for a font Blob it produced. Callers type-assert to discover it, keeping CSS generation out
// of the core FontProvider contract (interface segregation, per the brief's capability-interface
// pattern). familyDisplay is the family label to embed verbatim in the snippet.
type FontFaceRenderer interface {
	RenderFontFace(familyDisplay string, b Blob) string
}

// Fetch-by-identity note: get_icon/get_illustration/get_font fetch without a prior search. Rather
// than widen the core interface with a DirectResolver, the caller constructs the identifying Asset
// (Source + Title, plus render/variant hints in Ref) and passes it straight to Fetch. A dedicated
// DirectResolver capability interface can be introduced later if a provider needs to validate or
// enrich an identity before fetching; the embedded providers do not.

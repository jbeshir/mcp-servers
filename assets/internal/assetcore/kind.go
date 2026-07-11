// Package assetcore is the passive core toolkit for the assets server: immutable value types,
// per-kind provider interfaces, a read-after-build registry, and a cross-provider search
// aggregator. It contains no provider logic, no environment reads, and no network code; concrete
// providers live under internal/providers and depend on this package, never the reverse.
package assetcore

// Kind enumerates the asset kinds this server serves. Only the three embedded kinds exist today;
// further kinds (photo, audio) are added when their providers land.
type Kind string

const (
	// KindIcon is a single vector icon rendered to standalone SVG.
	KindIcon Kind = "icon"
	// KindIllustration is a standalone SVG illustration.
	KindIllustration Kind = "illustration"
	// KindFont is a font family variant (woff2, optionally with @font-face CSS).
	KindFont Kind = "font"
)

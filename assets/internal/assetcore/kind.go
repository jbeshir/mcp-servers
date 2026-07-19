// Package assetcore is the passive core toolkit for the assets server: immutable value types,
// per-kind provider interfaces, a read-after-build registry, and a cross-provider search
// aggregator. It contains no provider logic, no environment reads, and no network code; concrete
// providers live under internal/providers and depend on this package, never the reverse.
package assetcore

// Kind enumerates the asset kinds this server serves: icon, illustration, font, photo, texture,
// model, audio, and sprite.
type Kind string

const (
	// KindIcon is a single vector icon rendered to standalone SVG.
	KindIcon Kind = "icon"
	// KindIllustration is a standalone SVG illustration.
	KindIllustration Kind = "illustration"
	// KindFont is a font family variant (woff2, optionally with @font-face CSS).
	KindFont Kind = "font"
	// KindPhoto is a raster photograph.
	KindPhoto Kind = "photo"
	// KindTexture is a PBR material archive (a zip of texture maps at a given resolution/format).
	KindTexture Kind = "texture"
	// KindModel is a 3D model, delivered as a glTF/GLB file or a zip of a glTF plus its referenced
	// assets.
	KindModel Kind = "model"
	// KindAudio is an audio clip (an mp3 or ogg file).
	KindAudio Kind = "audio"
	// KindSprite is raster game art, including atlas-backed sub-sprites.
	KindSprite Kind = "sprite"
)

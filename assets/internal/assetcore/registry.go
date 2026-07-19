package assetcore

import (
	"context"
	"fmt"
	"sort"
)

// Registry maps provider name -> provider, segregated per kind. It is built once during wiring
// (config.Setup) via the Add* methods and treated read-only thereafter: the accessors take no lock
// and callers must not mutate it after startup. No init(), no globals, no blank imports.
type Registry struct {
	icons         map[string]IconProvider
	illustrations map[string]IllustrationProvider
	fonts         map[string]FontProvider
	photos        map[string]PhotoProvider
	textures      map[string]TextureProvider
	models        map[string]ModelProvider
	audio         map[string]AudioProvider
	sprites       map[string]SpriteProvider
}

// NewRegistry returns an empty registry ready for Add* calls.
func NewRegistry() *Registry {
	return &Registry{
		icons:         map[string]IconProvider{},
		illustrations: map[string]IllustrationProvider{},
		fonts:         map[string]FontProvider{},
		photos:        map[string]PhotoProvider{},
		textures:      map[string]TextureProvider{},
		models:        map[string]ModelProvider{},
		audio:         map[string]AudioProvider{},
		sprites:       map[string]SpriteProvider{},
	}
}

// AddIcon registers an icon provider under its Name. A later registration with the same name wins.
func (r *Registry) AddIcon(p IconProvider) { r.icons[p.Name()] = p }

// AddIllustration registers an illustration provider under its Name.
func (r *Registry) AddIllustration(p IllustrationProvider) { r.illustrations[p.Name()] = p }

// AddFont registers a font provider under its Name.
func (r *Registry) AddFont(p FontProvider) { r.fonts[p.Name()] = p }

// AddPhoto registers a photo provider under its Name.
func (r *Registry) AddPhoto(p PhotoProvider) { r.photos[p.Name()] = p }

// AddTexture registers a texture provider under its Name.
func (r *Registry) AddTexture(p TextureProvider) { r.textures[p.Name()] = p }

// AddModel registers a model provider under its Name.
func (r *Registry) AddModel(p ModelProvider) { r.models[p.Name()] = p }

// AddAudio registers an audio provider under its Name.
func (r *Registry) AddAudio(p AudioProvider) { r.audio[p.Name()] = p }

// AddSprite registers a sprite provider under its Name.
func (r *Registry) AddSprite(p SpriteProvider) { r.sprites[p.Name()] = p }

// Icons returns the registered icon providers ordered deterministically by name.
func (r *Registry) Icons() []IconProvider { return sortedProviders(r.icons) }

// Illustrations returns the registered illustration providers ordered deterministically by name.
func (r *Registry) Illustrations() []IllustrationProvider { return sortedProviders(r.illustrations) }

// Fonts returns the registered font providers ordered deterministically by name.
func (r *Registry) Fonts() []FontProvider { return sortedProviders(r.fonts) }

// Photos returns the registered photo providers ordered deterministically by name.
func (r *Registry) Photos() []PhotoProvider { return sortedProviders(r.photos) }

// Textures returns the registered texture providers ordered deterministically by name.
func (r *Registry) Textures() []TextureProvider { return sortedProviders(r.textures) }

// Models returns the registered model providers ordered deterministically by name.
func (r *Registry) Models() []ModelProvider { return sortedProviders(r.models) }

// Audio returns the registered audio providers ordered deterministically by name.
func (r *Registry) Audio() []AudioProvider { return sortedProviders(r.audio) }

// Sprites returns the registered sprite providers ordered deterministically by name.
func (r *Registry) Sprites() []SpriteProvider { return sortedProviders(r.sprites) }

// route resolves id's composite provider prefix against m and reports the provider-local remainder. A
// malformed id or an unknown provider name is reported as ErrNotFound.
func route[P Provider](m map[string]P, kind, id string) (p P, local string, err error) {
	name, local, ok := ParseAssetID(id)
	if !ok {
		return p, "", fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok = m[name]
	if !ok {
		return p, "", fmt.Errorf("%w: no %s provider %q", ErrNotFound, kind, name)
	}

	return p, local, nil
}

// FetchIcon routes id to the provider named in its composite prefix and fetches the icon by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchIcon(ctx context.Context, id string, opts IconFetchOpts) (Blob, error) {
	p, local, err := route(r.icons, "icon", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local, opts)
}

// FetchIllustration routes id to the provider named in its composite prefix and fetches the
// illustration by its provider-local id. A malformed id or an unknown provider is ErrNotFound.
func (r *Registry) FetchIllustration(ctx context.Context, id string) (Blob, error) {
	p, local, err := route(r.illustrations, "illustration", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local)
}

// FetchFont routes id to the provider named in its composite prefix and fetches the font by its
// provider-local id, returning that provider so the caller can type-assert an optional
// FontFaceRenderer for @font-face CSS. A malformed id or an unknown provider is ErrNotFound.
func (r *Registry) FetchFont(ctx context.Context, id string, opts FontFetchOpts) (Blob, FontProvider, error) {
	p, local, err := route(r.fonts, "font", id)
	if err != nil {
		return Blob{}, nil, err
	}

	b, err := p.Fetch(ctx, local, opts)
	if err != nil {
		return Blob{}, nil, err
	}

	return b, p, nil
}

// FetchPhoto routes id to the provider named in its composite prefix and fetches the photo by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchPhoto(ctx context.Context, id string, opts PhotoFetchOpts) (Blob, error) {
	p, local, err := route(r.photos, "photo", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local, opts)
}

// FetchTexture routes id to the provider named in its composite prefix and fetches the texture by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchTexture(ctx context.Context, id string, opts TextureFetchOpts) (Blob, error) {
	p, local, err := route(r.textures, "texture", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local, opts)
}

// FetchModel routes id to the provider named in its composite prefix and fetches the model by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchModel(ctx context.Context, id string, opts ModelFetchOpts) (Blob, error) {
	p, local, err := route(r.models, "model", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local, opts)
}

// FetchAudio routes id to the provider named in its composite prefix and fetches the audio by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchAudio(ctx context.Context, id string, opts AudioFetchOpts) (Blob, error) {
	p, local, err := route(r.audio, "audio", id)
	if err != nil {
		return Blob{}, err
	}

	return p.Fetch(ctx, local, opts)
}

// FetchSprite routes id to its sprite provider.
func (r *Registry) FetchSprite(ctx context.Context, id string, opts SpriteFetchOpts) (Blob, error) {
	p, local, err := route(r.sprites, "sprite", id)
	if err != nil {
		return Blob{}, err
	}
	return p.Fetch(ctx, local, opts)
}

// ProviderInfo describes a registered provider and, when it implements SourceLister, the upstream
// sources it serves. Sources is nil for providers that cannot enumerate their catalogue.
type ProviderInfo struct {
	Name    string
	Kind    Kind
	Sources []Source
}

// Providers returns every registered provider across all kinds, sorted by (kind, name), each carrying
// its Sources() if it implements SourceLister. It backs the list_asset_sources discovery tool.
func (r *Registry) Providers() []ProviderInfo {
	var infos []ProviderInfo

	collect := func(p Provider) {
		info := ProviderInfo{Name: p.Name(), Kind: p.Kind()}
		if sl, ok := p.(SourceLister); ok {
			info.Sources = sl.Sources()
		}
		infos = append(infos, info)
	}

	for _, p := range r.Icons() {
		collect(p)
	}
	for _, p := range r.Illustrations() {
		collect(p)
	}
	for _, p := range r.Fonts() {
		collect(p)
	}
	for _, p := range r.Photos() {
		collect(p)
	}
	for _, p := range r.Textures() {
		collect(p)
	}
	for _, p := range r.Models() {
		collect(p)
	}
	for _, p := range r.Audio() {
		collect(p)
	}
	for _, p := range r.Sprites() {
		collect(p)
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Kind != infos[j].Kind {
			return infos[i].Kind < infos[j].Kind
		}

		return infos[i].Name < infos[j].Name
	})

	return infos
}

// sortedKeys returns the map keys sorted ascending, giving deterministic provider ordering.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

// sortedProviders returns the map values ordered by their key sorted ascending, giving deterministic
// provider ordering.
func sortedProviders[P any](m map[string]P) []P {
	names := sortedKeys(m)
	out := make([]P, len(names))
	for i, n := range names {
		out[i] = m[n]
	}

	return out
}

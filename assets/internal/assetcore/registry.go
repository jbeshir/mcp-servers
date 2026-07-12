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
}

// NewRegistry returns an empty registry ready for Add* calls.
func NewRegistry() *Registry {
	return &Registry{
		icons:         map[string]IconProvider{},
		illustrations: map[string]IllustrationProvider{},
		fonts:         map[string]FontProvider{},
		photos:        map[string]PhotoProvider{},
		textures:      map[string]TextureProvider{},
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

// Icons returns the registered icon providers ordered deterministically by name.
func (r *Registry) Icons() []IconProvider {
	names := sortedKeys(r.icons)
	out := make([]IconProvider, len(names))
	for i, n := range names {
		out[i] = r.icons[n]
	}

	return out
}

// Illustrations returns the registered illustration providers ordered deterministically by name.
func (r *Registry) Illustrations() []IllustrationProvider {
	names := sortedKeys(r.illustrations)
	out := make([]IllustrationProvider, len(names))
	for i, n := range names {
		out[i] = r.illustrations[n]
	}

	return out
}

// Fonts returns the registered font providers ordered deterministically by name.
func (r *Registry) Fonts() []FontProvider {
	names := sortedKeys(r.fonts)
	out := make([]FontProvider, len(names))
	for i, n := range names {
		out[i] = r.fonts[n]
	}

	return out
}

// Photos returns the registered photo providers ordered deterministically by name.
func (r *Registry) Photos() []PhotoProvider {
	names := sortedKeys(r.photos)
	out := make([]PhotoProvider, len(names))
	for i, n := range names {
		out[i] = r.photos[n]
	}

	return out
}

// Textures returns the registered texture providers ordered deterministically by name.
func (r *Registry) Textures() []TextureProvider {
	names := sortedKeys(r.textures)
	out := make([]TextureProvider, len(names))
	for i, n := range names {
		out[i] = r.textures[n]
	}

	return out
}

// FetchIcon routes id to the provider named in its composite prefix and fetches the icon by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchIcon(ctx context.Context, id string, opts IconFetchOpts) (Blob, error) {
	name, local, ok := ParseAssetID(id)
	if !ok {
		return Blob{}, fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok := r.icons[name]
	if !ok {
		return Blob{}, fmt.Errorf("%w: no icon provider %q", ErrNotFound, name)
	}

	return p.Fetch(ctx, local, opts)
}

// FetchIllustration routes id to the provider named in its composite prefix and fetches the
// illustration by its provider-local id. A malformed id or an unknown provider is ErrNotFound.
func (r *Registry) FetchIllustration(ctx context.Context, id string) (Blob, error) {
	name, local, ok := ParseAssetID(id)
	if !ok {
		return Blob{}, fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok := r.illustrations[name]
	if !ok {
		return Blob{}, fmt.Errorf("%w: no illustration provider %q", ErrNotFound, name)
	}

	return p.Fetch(ctx, local)
}

// FetchFont routes id to the provider named in its composite prefix and fetches the font by its
// provider-local id, returning that provider so the caller can type-assert an optional
// FontFaceRenderer for @font-face CSS. A malformed id or an unknown provider is ErrNotFound.
func (r *Registry) FetchFont(ctx context.Context, id string, opts FontFetchOpts) (Blob, FontProvider, error) {
	name, local, ok := ParseAssetID(id)
	if !ok {
		return Blob{}, nil, fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok := r.fonts[name]
	if !ok {
		return Blob{}, nil, fmt.Errorf("%w: no font provider %q", ErrNotFound, name)
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
	name, local, ok := ParseAssetID(id)
	if !ok {
		return Blob{}, fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok := r.photos[name]
	if !ok {
		return Blob{}, fmt.Errorf("%w: no photo provider %q", ErrNotFound, name)
	}

	return p.Fetch(ctx, local, opts)
}

// FetchTexture routes id to the provider named in its composite prefix and fetches the texture by its
// provider-local id. A malformed id or an unknown provider name is reported as ErrNotFound.
func (r *Registry) FetchTexture(ctx context.Context, id string, opts TextureFetchOpts) (Blob, error) {
	name, local, ok := ParseAssetID(id)
	if !ok {
		return Blob{}, fmt.Errorf("%w: malformed asset id %q", ErrNotFound, id)
	}

	p, ok := r.textures[name]
	if !ok {
		return Blob{}, fmt.Errorf("%w: no texture provider %q", ErrNotFound, name)
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

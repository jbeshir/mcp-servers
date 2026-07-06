package assetcore

import "sort"

// Registry maps provider name -> provider, segregated per kind. It is built once during wiring
// (config.Setup) via the Add* methods and treated read-only thereafter: the accessors take no lock
// and callers must not mutate it after startup. No init(), no globals, no blank imports.
type Registry struct {
	icons         map[string]IconProvider
	illustrations map[string]IllustrationProvider
	fonts         map[string]FontProvider
}

// NewRegistry returns an empty registry ready for Add* calls.
func NewRegistry() *Registry {
	return &Registry{
		icons:         map[string]IconProvider{},
		illustrations: map[string]IllustrationProvider{},
		fonts:         map[string]FontProvider{},
	}
}

// AddIcon registers an icon provider under its Name. A later registration with the same name wins.
func (r *Registry) AddIcon(p IconProvider) { r.icons[p.Name()] = p }

// AddIllustration registers an illustration provider under its Name.
func (r *Registry) AddIllustration(p IllustrationProvider) { r.illustrations[p.Name()] = p }

// AddFont registers a font provider under its Name.
func (r *Registry) AddFont(p FontProvider) { r.fonts[p.Name()] = p }

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

// sortedKeys returns the map keys sorted ascending, giving deterministic provider ordering.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

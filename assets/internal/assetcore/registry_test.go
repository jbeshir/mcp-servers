package assetcore

import (
	"context"
	"errors"
	"testing"
)

// embeddedIconsName is a sample provider name reused across these tests.
const embeddedIconsName = "embedded-icons"

// fakeIconProvider is a minimal IconProvider used across the assetcore tests. A non-nil err makes
// both Search and Fetch fail; otherwise Search returns assets and Fetch echoes the provider name and
// the local id it was given.
type fakeIconProvider struct {
	name   string
	assets []Asset
	err    error
}

func (f fakeIconProvider) Name() string { return f.name }
func (f fakeIconProvider) Kind() Kind   { return KindIcon }

func (f fakeIconProvider) Search(_ context.Context, _ SearchOpts) ([]Asset, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.assets, nil
}

func (f fakeIconProvider) Fetch(_ context.Context, id string, _ IconFetchOpts) (Blob, error) {
	if f.err != nil {
		return Blob{}, f.err
	}

	return Blob{Asset: Asset{ID: id}, Content: []byte(f.name)}, nil
}

// sourcedIconProvider adds the SourceLister capability to fakeIconProvider.
type sourcedIconProvider struct {
	fakeIconProvider
	sources []Source
}

func (f sourcedIconProvider) Sources() []Source { return f.sources }

// fakeFontProvider is a minimal FontProvider for the routed-fetch and Providers tests.
type fakeFontProvider struct {
	name string
	err  error
}

func (f fakeFontProvider) Name() string { return f.name }
func (f fakeFontProvider) Kind() Kind   { return KindFont }

func (f fakeFontProvider) Search(_ context.Context, _ SearchOpts) ([]Asset, error) {
	return nil, f.err
}

func (f fakeFontProvider) Fetch(_ context.Context, id string, _ FontFetchOpts) (Blob, error) {
	if f.err != nil {
		return Blob{}, f.err
	}

	return Blob{Asset: Asset{ID: id}, Content: []byte(f.name)}, nil
}

// fakeIllustrationProvider is a minimal IllustrationProvider for the routed-fetch tests.
type fakeIllustrationProvider struct {
	name string
	err  error
}

func (f fakeIllustrationProvider) Name() string { return f.name }
func (f fakeIllustrationProvider) Kind() Kind   { return KindIllustration }

func (f fakeIllustrationProvider) Search(_ context.Context, _ SearchOpts) ([]Asset, error) {
	return nil, f.err
}

func (f fakeIllustrationProvider) Fetch(_ context.Context, id string) (Blob, error) {
	if f.err != nil {
		return Blob{}, f.err
	}

	return Blob{Asset: Asset{ID: id}, Content: []byte(f.name)}, nil
}

func TestRegistryIconsDeterministicOrder(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: embeddedIconsName})
	r.AddIcon(fakeIconProvider{name: "aardvark"})
	r.AddIcon(fakeIconProvider{name: "zzz"})

	want := []string{"aardvark", embeddedIconsName, "zzz"}

	// Two calls must yield the same deterministic order.
	for range 2 {
		got := r.Icons()
		if len(got) != len(want) {
			t.Fatalf("Icons() length = %d, want %d", len(got), len(want))
		}
		for i, p := range got {
			if p.Name() != want[i] {
				t.Errorf("Icons()[%d] = %q, want %q", i, p.Name(), want[i])
			}
		}
	}
}

func TestRegistryAddIconSameNameWins(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "dup", assets: []Asset{{ID: "dup:a"}}})
	r.AddIcon(fakeIconProvider{name: "dup", assets: []Asset{{ID: "dup:b"}}})

	got := r.Icons()
	if len(got) != 1 {
		t.Fatalf("Icons() length = %d, want 1", len(got))
	}

	assets, _ := got[0].Search(t.Context(), SearchOpts{})
	if len(assets) != 1 || assets[0].ID != "dup:b" {
		t.Errorf("second registration did not win: assets = %+v, want a single asset with ID dup:b", assets)
	}
}

func TestRegistryKindsSegregated(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: embeddedIconsName})

	if len(r.Icons()) != 1 {
		t.Errorf("Icons() length = %d, want 1", len(r.Icons()))
	}
	if len(r.Illustrations()) != 0 {
		t.Errorf("Illustrations() length = %d, want 0", len(r.Illustrations()))
	}
	if len(r.Fonts()) != 0 {
		t.Errorf("Fonts() length = %d, want 0", len(r.Fonts()))
	}
}

func TestFetchIconRoutesToProvider(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a"})
	r.AddIcon(fakeIconProvider{name: "b"})

	blob, err := r.FetchIcon(t.Context(), AssetID("b", "some/local"), IconFetchOpts{})
	if err != nil {
		t.Fatalf("FetchIcon error = %v, want nil", err)
	}
	if string(blob.Content) != "b" {
		t.Errorf("FetchIcon routed to %q, want provider b", string(blob.Content))
	}
	// The provider must receive the local id, not the composite.
	if blob.Asset.ID != "some/local" {
		t.Errorf("provider saw id %q, want the provider-local %q", blob.Asset.ID, "some/local")
	}
}

func TestFetchIconMalformedID(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a"})

	if _, err := r.FetchIcon(t.Context(), "no-colon-here", IconFetchOpts{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIcon(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchIconUnknownProvider(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a"})

	if _, err := r.FetchIcon(t.Context(), AssetID("z", "x"), IconFetchOpts{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIcon(unknown provider) error = %v, want ErrNotFound", err)
	}
}

func TestFetchIllustrationRoutesToProvider(t *testing.T) {
	r := NewRegistry()
	r.AddIllustration(fakeIllustrationProvider{name: "col"})

	blob, err := r.FetchIllustration(t.Context(), AssetID("col", "set/name"))
	if err != nil {
		t.Fatalf("FetchIllustration error = %v, want nil", err)
	}
	if string(blob.Content) != "col" {
		t.Errorf("FetchIllustration routed to %q, want provider col", string(blob.Content))
	}

	if _, err := r.FetchIllustration(t.Context(), AssetID("nope", "x")); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIllustration(unknown provider) error = %v, want ErrNotFound", err)
	}
	if _, err := r.FetchIllustration(t.Context(), "malformed"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIllustration(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchFontRoutesAndReturnsProvider(t *testing.T) {
	r := NewRegistry()
	want := fakeFontProvider{name: "fonts"}
	r.AddFont(want)

	blob, prov, err := r.FetchFont(t.Context(), AssetID("fonts", "inter"), FontFetchOpts{})
	if err != nil {
		t.Fatalf("FetchFont error = %v, want nil", err)
	}
	if string(blob.Content) != "fonts" {
		t.Errorf("FetchFont routed to %q, want provider fonts", string(blob.Content))
	}
	if prov == nil || prov.Name() != "fonts" {
		t.Errorf("FetchFont returned provider %v, want the fonts provider", prov)
	}

	if _, _, err := r.FetchFont(t.Context(), AssetID("z", "x"), FontFetchOpts{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchFont(unknown provider) error = %v, want ErrNotFound", err)
	}
	if _, _, err := r.FetchFont(t.Context(), "malformed", FontFetchOpts{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchFont(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchFontPropagatesProviderError(t *testing.T) {
	r := NewRegistry()
	r.AddFont(fakeFontProvider{name: "fonts", err: errors.New("boom")})

	if _, _, err := r.FetchFont(t.Context(), AssetID("fonts", "inter"), FontFetchOpts{}); err == nil {
		t.Error("FetchFont error = nil, want the provider's error")
	}
}

func TestProvidersShapeAndSort(t *testing.T) {
	r := NewRegistry()
	// Register out of (kind, name) order to prove Providers() sorts.
	r.AddFont(fakeFontProvider{name: "embedded-fonts"})
	r.AddIcon(sourcedIconProvider{
		fakeIconProvider: fakeIconProvider{name: "embedded-icons"},
		sources:          []Source{{Name: "lucide", License: License{SPDX: "ISC"}, Count: 3}},
	})
	r.AddIllustration(fakeIllustrationProvider{name: "embedded-illustrations"})

	infos := r.Providers()
	if len(infos) != 3 {
		t.Fatalf("Providers() length = %d, want 3", len(infos))
	}

	// Sorted by (kind, name): KindFont < KindIcon < KindIllustration lexically ("font" < "icon" < "illustration").
	wantKinds := []Kind{KindFont, KindIcon, KindIllustration}
	for i, want := range wantKinds {
		if infos[i].Kind != want {
			t.Errorf("Providers()[%d].Kind = %q, want %q", i, infos[i].Kind, want)
		}
	}

	// The SourceLister provider carries its sources; the others carry nil.
	icon := infos[1]
	if icon.Name != "embedded-icons" {
		t.Fatalf("Providers()[1].Name = %q, want embedded-icons", icon.Name)
	}
	if len(icon.Sources) != 1 || icon.Sources[0].Name != "lucide" || icon.Sources[0].Count != 3 {
		t.Errorf("icon.Sources = %+v, want one lucide source with count 3", icon.Sources)
	}
	if infos[0].Sources != nil {
		t.Errorf("font provider Sources = %+v, want nil (no SourceLister)", infos[0].Sources)
	}
}

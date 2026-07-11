package assetcore_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

// embeddedIconsName is a sample provider name reused across these tests.
const embeddedIconsName = "embedded-icons"

func TestRegistryIconsDeterministicOrder(t *testing.T) {
	r := assetcore.NewRegistry()
	r.AddIcon(newIconProvider(t, embeddedIconsName))
	r.AddIcon(newIconProvider(t, "aardvark"))
	r.AddIcon(newIconProvider(t, "zzz"))

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
	r := assetcore.NewRegistry()
	// The first registration is overwritten; only its Name is read (at registration).
	r.AddIcon(newIconProvider(t, "dup"))

	winner := newIconProvider(t, "dup")
	winner.EXPECT().Search(mock.Anything, mock.Anything).Return([]assetcore.Asset{{ID: "dup:b"}}, nil)
	r.AddIcon(winner)

	got := r.Icons()
	if len(got) != 1 {
		t.Fatalf("Icons() length = %d, want 1", len(got))
	}

	assets, _ := got[0].Search(t.Context(), assetcore.SearchOpts{})
	if len(assets) != 1 || assets[0].ID != "dup:b" {
		t.Errorf("second registration did not win: assets = %+v, want a single asset with ID dup:b", assets)
	}
}

func TestRegistryKindsSegregated(t *testing.T) {
	r := assetcore.NewRegistry()
	r.AddIcon(newIconProvider(t, embeddedIconsName))

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
	r := assetcore.NewRegistry()
	r.AddIcon(newIconProvider(t, "a"))

	b := newIconProvider(t, "b")
	expectIconFetchEcho(b, "b")
	r.AddIcon(b)

	blob, err := r.FetchIcon(t.Context(), assetcore.AssetID("b", "some/local"), assetcore.IconFetchOpts{})
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
	r := assetcore.NewRegistry()
	r.AddIcon(newIconProvider(t, "a"))

	if _, err := r.FetchIcon(t.Context(), "no-colon-here", assetcore.IconFetchOpts{}); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchIcon(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchIconUnknownProvider(t *testing.T) {
	r := assetcore.NewRegistry()
	r.AddIcon(newIconProvider(t, "a"))

	if _, err := r.FetchIcon(t.Context(), assetcore.AssetID("z", "x"), assetcore.IconFetchOpts{}); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchIcon(unknown provider) error = %v, want ErrNotFound", err)
	}
}

func TestFetchIllustrationRoutesToProvider(t *testing.T) {
	r := assetcore.NewRegistry()
	col := newIllustrationProvider(t, "col")
	expectIllustrationFetchEcho(col, "col")
	r.AddIllustration(col)

	blob, err := r.FetchIllustration(t.Context(), assetcore.AssetID("col", "set/name"))
	if err != nil {
		t.Fatalf("FetchIllustration error = %v, want nil", err)
	}
	if string(blob.Content) != "col" {
		t.Errorf("FetchIllustration routed to %q, want provider col", string(blob.Content))
	}

	if _, err := r.FetchIllustration(t.Context(), assetcore.AssetID("nope", "x")); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchIllustration(unknown provider) error = %v, want ErrNotFound", err)
	}
	if _, err := r.FetchIllustration(t.Context(), "malformed"); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchIllustration(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchFontRoutesAndReturnsProvider(t *testing.T) {
	r := assetcore.NewRegistry()
	f := newFontProvider(t, "fonts")
	expectFontFetchEcho(f, "fonts")
	r.AddFont(f)

	blob, prov, err := r.FetchFont(t.Context(), assetcore.AssetID("fonts", "inter"), assetcore.FontFetchOpts{})
	if err != nil {
		t.Fatalf("FetchFont error = %v, want nil", err)
	}
	if string(blob.Content) != "fonts" {
		t.Errorf("FetchFont routed to %q, want provider fonts", string(blob.Content))
	}
	if prov == nil || prov.Name() != "fonts" {
		t.Errorf("FetchFont returned provider %v, want the fonts provider", prov)
	}

	if _, _, err := r.FetchFont(t.Context(), assetcore.AssetID("z", "x"), assetcore.FontFetchOpts{}); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchFont(unknown provider) error = %v, want ErrNotFound", err)
	}
	if _, _, err := r.FetchFont(t.Context(), "malformed", assetcore.FontFetchOpts{}); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("FetchFont(malformed) error = %v, want ErrNotFound", err)
	}
}

func TestFetchFontPropagatesProviderError(t *testing.T) {
	r := assetcore.NewRegistry()
	f := newFontProvider(t, "fonts")
	f.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(assetcore.Blob{}, errors.New("boom"))
	r.AddFont(f)

	if _, _, err := r.FetchFont(t.Context(), assetcore.AssetID("fonts", "inter"), assetcore.FontFetchOpts{}); err == nil {
		t.Error("FetchFont error = nil, want the provider's error")
	}
}

func TestProvidersShapeAndSort(t *testing.T) {
	r := assetcore.NewRegistry()

	// Register out of (kind, name) order to prove Providers() sorts. Providers() reads each provider's
	// Kind, so each mock expects a Kind call.
	fontMock := newFontProvider(t, "embedded-fonts")
	fontMock.EXPECT().Kind().Return(assetcore.KindFont)
	r.AddFont(fontMock)

	iconMock := newIconProvider(t, "embedded-icons")
	iconMock.EXPECT().Kind().Return(assetcore.KindIcon)
	r.AddIcon(sourcedIconProvider{
		IconProvider: iconMock,
		sources:      []assetcore.Source{{Name: "lucide", License: assetcore.License{SPDX: "ISC"}, Count: 3}},
	})

	illusMock := newIllustrationProvider(t, "embedded-illustrations")
	illusMock.EXPECT().Kind().Return(assetcore.KindIllustration)
	r.AddIllustration(illusMock)

	infos := r.Providers()
	if len(infos) != 3 {
		t.Fatalf("Providers() length = %d, want 3", len(infos))
	}

	// Sorted by (kind, name): KindFont < KindIcon < KindIllustration lexically ("font" < "icon" < "illustration").
	wantKinds := []assetcore.Kind{assetcore.KindFont, assetcore.KindIcon, assetcore.KindIllustration}
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

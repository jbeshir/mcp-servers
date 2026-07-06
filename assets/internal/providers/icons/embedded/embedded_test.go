package embedded

import (
	"errors"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
)

// setLucide is the icon set exercised by these tests.
const setLucide = "lucide"

func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	c, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	return New(c)
}

func TestSearchMapsHitsToAssets(t *testing.T) {
	p := newTestProvider(t)

	page, err := p.Search(t.Context(), assetcore.IconQuery{
		SearchOpts: assetcore.SearchOpts{Query: "arrow", Limit: 10},
		Set:        setLucide,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(page.Assets) == 0 {
		t.Fatal("Search returned no assets, want at least one")
	}

	for _, a := range page.Assets {
		if a.Provider != providerName {
			t.Errorf("asset.Provider = %q, want %q", a.Provider, providerName)
		}
		if a.Source != setLucide {
			t.Errorf("asset.Source = %q, want %q", a.Source, setLucide)
		}
		if a.Kind != assetcore.KindIcon {
			t.Errorf("asset.Kind = %q, want %q", a.Kind, assetcore.KindIcon)
		}
		if a.Title == "" {
			t.Error("asset.Title is empty")
		}
	}
}

func TestFetchRendersSVGWithLicense(t *testing.T) {
	p := newTestProvider(t)

	blob, err := p.Fetch(t.Context(), assetcore.Asset{
		Source: setLucide,
		Title:  "a-arrow-down",
		Ref:    map[string]string{assetcore.RefColor: "#ff0000", assetcore.RefSize: "32"},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	out := string(blob.Content)
	for _, want := range []string{"<svg ", `width="32"`, `height="32"`, `color="#ff0000"`} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered SVG missing %q: %s", want, out)
		}
	}

	if blob.ContentType != "image/svg+xml" {
		t.Errorf("blob.ContentType = %q, want image/svg+xml", blob.ContentType)
	}
	// lucide is catalogued as ISC; the adapter must carry that through from the catalog.
	if blob.Asset.License.SPDX != "ISC" {
		t.Errorf("blob.Asset.License.SPDX = %q, want %q", blob.Asset.License.SPDX, "ISC")
	}
}

func TestFetchUnknownIconIsNotFound(t *testing.T) {
	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), assetcore.Asset{
		Source: setLucide,
		Title:  "definitely-not-an-icon",
	})
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("Fetch unknown icon error = %v, want assetcore.ErrNotFound", err)
	}
}

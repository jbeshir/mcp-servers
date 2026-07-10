package embeddedicons

import (
	"errors"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
)

// setLucide is the icon set exercised by these tests.
const setLucide = "lucide"

func TestRenderLucideArrow(t *testing.T) {
	svg, err := renderIcon("lucide", "a-arrow-down", "", 0)
	if err != nil {
		t.Fatalf("renderIcon() error = %v, want nil", err)
	}
	out := string(svg)
	if !strings.Contains(out, `viewBox="0 0 24 24"`) {
		t.Errorf("renderIcon() output missing viewBox: %s", out)
	}
	if !strings.Contains(out, "<path") {
		t.Errorf("renderIcon() output missing <path: %s", out)
	}
	if !strings.HasPrefix(out, "<svg ") || !strings.HasSuffix(out, "</svg>") {
		t.Errorf("renderIcon() output not wrapped in <svg>...</svg>: %s", out)
	}
}

func TestRenderColorAndSize(t *testing.T) {
	svg, err := renderIcon("lucide", "a-arrow-down", "#ff0000", 32)
	if err != nil {
		t.Fatalf("renderIcon() error = %v, want nil", err)
	}
	out := string(svg)
	for _, want := range []string{`width="32"`, `height="32"`, `viewBox="0 0 24 24"`, `color="#ff0000"`} {
		if !strings.Contains(out, want) {
			t.Errorf("renderIcon() output missing %q: %s", want, out)
		}
	}
}

func TestRenderColorEscaped(t *testing.T) {
	svg, err := renderIcon("lucide", "a-arrow-down", `"><script>x</script>`, 0)
	if err != nil {
		t.Fatalf("renderIcon() error = %v, want nil", err)
	}
	out := string(svg)
	if strings.Contains(out, `"><script>`) {
		t.Errorf("renderIcon() output contains unescaped breakout: %s", out)
	}
	if !strings.Contains(out, "&quot;&gt;&lt;script&gt;") {
		t.Errorf("renderIcon() output missing escaped color value: %s", out)
	}
	if !strings.HasPrefix(out, "<svg ") || !strings.HasSuffix(out, "</svg>") {
		t.Errorf("renderIcon() output not wrapped in <svg>...</svg>: %s", out)
	}
}

func TestRenderBootstrapIconsGrid(t *testing.T) {
	svg, err := renderIcon("bootstrap-icons", "alarm", "", 0)
	if err != nil {
		t.Fatalf("renderIcon() error = %v, want nil", err)
	}
	if !strings.Contains(string(svg), `viewBox="0 0 16 16"`) {
		t.Errorf("renderIcon() output missing 16x16 viewBox: %s", svg)
	}
}

func TestRenderPhosphorGrid(t *testing.T) {
	svg, err := renderIcon("phosphor", "acorn", "", 0)
	if err != nil {
		t.Fatalf("renderIcon() error = %v, want nil", err)
	}
	if !strings.Contains(string(svg), `viewBox="0 0 256 256"`) {
		t.Errorf("renderIcon() output missing 256x256 viewBox: %s", svg)
	}
}

func TestSearchLucideArrow(t *testing.T) {
	results := searchIcons("arrow", "lucide", 10)
	if len(results) == 0 {
		t.Fatal("searchIcons() returned no results, want at least one")
	}
	for _, m := range results {
		if m.set != "lucide" {
			t.Errorf("searchIcons() result set = %q, want %q", m.set, "lucide")
		}
	}
}

func TestRenderNotFound(t *testing.T) {
	if _, err := renderIcon("lucide", "definitely-not-an-icon", "", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("renderIcon() unknown icon error = %v, want ErrNotFound", err)
	}
	if _, err := renderIcon("definitely-not-a-set", "a-arrow-down", "", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("renderIcon() unknown set error = %v, want ErrNotFound", err)
	}
}

func TestSets(t *testing.T) {
	want := []string{
		"bootstrap-icons", "feather", "heroicons", "lucide",
		"material-symbols", "phosphor", "simple-icons", "tabler",
	}
	got := loadedSetNames()
	if len(got) != len(want) {
		t.Fatalf("loadedSetNames() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("loadedSetNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

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
	// lucide is catalogued as ISC; the provider must carry that through from the catalog.
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

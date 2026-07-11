package embeddedicons

import (
	"errors"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
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
	results := searchIcons("arrow", assetcore.Filter{Only: []string{"lucide"}}, 10)
	if len(results) == 0 {
		t.Fatal("searchIcons() returned no results, want at least one")
	}
	for _, m := range results {
		if m.set != "lucide" {
			t.Errorf("searchIcons() result set = %q, want %q", m.set, "lucide")
		}
	}
}

func TestSearchExcludeSource(t *testing.T) {
	// Excluding lucide must yield no lucide hits even though the query would otherwise match them.
	results := searchIcons("arrow", assetcore.Filter{Except: []string{"lucide"}}, 200)
	for _, m := range results {
		if m.set == "lucide" {
			t.Errorf("searchIcons() with lucide excluded still returned a lucide hit: %+v", m)
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

func TestSearchMapsHitsToAssets(t *testing.T) {
	p := New()

	page, err := p.Search(t.Context(), assetcore.SearchOpts{
		Query:   "arrow",
		Limit:   10,
		Sources: assetcore.Filter{Only: []string{setLucide}},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(page.Assets) == 0 {
		t.Fatal("Search returned no assets, want at least one")
	}

	for _, a := range page.Assets {
		provider, local, ok := assetcore.ParseAssetID(a.ID)
		if !ok || provider != providerName {
			t.Errorf("asset.ID = %q, want composite id under %q", a.ID, providerName)
		}
		if !strings.HasPrefix(local, setLucide+"/") {
			t.Errorf("asset local id = %q, want %s/ prefix", local, setLucide)
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
	p := New()

	blob, err := p.Fetch(t.Context(), setLucide+"/a-arrow-down", assetcore.IconFetchOpts{Color: "#ff0000", Size: 32})
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
	// lucide is licensed ISC; the provider owns and carries that through.
	if blob.Asset.License.SPDX != "ISC" {
		t.Errorf("blob.Asset.License.SPDX = %q, want %q", blob.Asset.License.SPDX, "ISC")
	}
	if blob.Asset.ID != assetcore.AssetID(providerName, setLucide+"/a-arrow-down") {
		t.Errorf("blob.Asset.ID = %q, want composite id", blob.Asset.ID)
	}
}

func TestFetchMalformedIDIsNotFound(t *testing.T) {
	p := New()

	if _, err := p.Fetch(t.Context(), "no-slash", assetcore.IconFetchOpts{}); !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("Fetch(malformed) error = %v, want assetcore.ErrNotFound", err)
	}
}

func TestFetchUnknownIconIsNotFound(t *testing.T) {
	p := New()

	_, err := p.Fetch(t.Context(), setLucide+"/definitely-not-an-icon", assetcore.IconFetchOpts{})
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("Fetch unknown icon error = %v, want assetcore.ErrNotFound", err)
	}
}

func TestSourcesReportsSetsWithLicenseAndCount(t *testing.T) {
	p := New()

	srcs := p.Sources()
	if len(srcs) != 8 {
		t.Fatalf("Sources() length = %d, want 8", len(srcs))
	}

	byName := make(map[string]assetcore.Source, len(srcs))
	for _, s := range srcs {
		byName[s.Name] = s
	}

	lucide, ok := byName["lucide"]
	if !ok {
		t.Fatal("Sources() missing lucide")
	}
	if lucide.License.SPDX != "ISC" {
		t.Errorf("lucide license = %q, want ISC", lucide.License.SPDX)
	}
	if lucide.Count <= 0 {
		t.Errorf("lucide count = %d, want a positive count derived from embedded data", lucide.Count)
	}
	if got := byName["simple-icons"].License.SPDX; got != "CC0-1.0" {
		t.Errorf("simple-icons license = %q, want CC0-1.0", got)
	}
	if got := byName["material-symbols"].License.SPDX; got != "Apache-2.0" {
		t.Errorf("material-symbols license = %q, want Apache-2.0", got)
	}
	if got := byName["feather"].License.SPDX; got != "MIT" {
		t.Errorf("feather license = %q, want MIT", got)
	}
}

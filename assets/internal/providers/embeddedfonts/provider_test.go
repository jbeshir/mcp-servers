package embeddedfonts

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

const (
	knownFamily   = "Inter"
	knownSlug     = "inter"
	knownFilename = "inter-latin-400-normal.woff2"
)

func TestGetKnownFamilyByDisplayName(t *testing.T) {
	f, err := getFont(knownFamily, 400, "normal")
	if err != nil {
		t.Fatalf("getFont(%q, 400, %q) returned unexpected error: %v", knownFamily, styleNormal, err)
	}
	if len(f.data) == 0 {
		t.Fatalf("getFont(%q, 400, %q) returned empty data", knownFamily, styleNormal)
	}
	if f.filename != knownFilename {
		t.Fatalf("getFont(%q, 400, %q).filename = %q, want %q", knownFamily, styleNormal, f.filename, knownFilename)
	}
	if !bytes.HasPrefix(f.data, []byte("wOF2")) {
		t.Fatalf("getFont(%q, 400, %q).data does not start with the woff2 magic bytes wOF2", knownFamily, styleNormal)
	}
}

func TestGetKnownFamilyBySlug(t *testing.T) {
	f, err := getFont(knownSlug, 700, "normal")
	if err != nil {
		t.Fatalf("getFont(%q, 700, %q) returned unexpected error: %v", knownSlug, styleNormal, err)
	}
	if len(f.data) == 0 {
		t.Fatalf("getFont(%q, 700, %q) returned empty data", knownSlug, styleNormal)
	}
	if want := "inter-latin-700-normal.woff2"; f.filename != want {
		t.Fatalf("getFont(%q, 700, %q).filename = %q, want %q", knownSlug, styleNormal, f.filename, want)
	}
}

func TestFontFace(t *testing.T) {
	f, err := getFont(knownFamily, 400, "normal")
	if err != nil {
		t.Fatalf("getFont(%q, 400, %q) returned unexpected error: %v", knownFamily, styleNormal, err)
	}

	css := fontFaceCSS(knownFamily, f)
	for _, want := range []string{"@font-face", "font-weight: 400", knownFilename} {
		if !strings.Contains(css, want) {
			t.Errorf("fontFaceCSS(%q, f) = %q, want it to contain %q", knownFamily, css, want)
		}
	}
}

func TestParseFontFilename(t *testing.T) {
	weight, style := parseFontFilename("inter-latin-700-normal.woff2")
	if weight != 700 || style != "normal" {
		t.Errorf("parseFontFilename = (%d, %q), want (700, normal)", weight, style)
	}

	if w, s := parseFontFilename("garbage.woff2"); w != 0 || s != "" {
		t.Errorf("parseFontFilename(garbage) = (%d, %q), want (0, \"\")", w, s)
	}
}

func TestRenderFontFaceFromBlob(t *testing.T) {
	p := New()

	blob, err := p.Fetch(t.Context(), knownSlug, assetcore.FontFetchOpts{Weight: 700})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	css := p.RenderFontFace(knownFamily, blob)
	for _, want := range []string{"@font-face", "font-weight: 700", "inter-latin-700-normal.woff2"} {
		if !strings.Contains(css, want) {
			t.Errorf("RenderFontFace CSS = %q, want it to contain %q", css, want)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	tests := []struct {
		name   string
		family string
		weight int
		style  string
	}{
		{name: "missing weight", family: knownFamily, weight: 123, style: "normal"},
		{name: "unknown family", family: "Nope", weight: 400, style: "normal"},
		{name: "missing style", family: knownFamily, weight: 400, style: "italic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getFont(tt.family, tt.weight, tt.style)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("getFont(%q, %d, %q) error = %v, want ErrNotFound", tt.family, tt.weight, tt.style, err)
			}
		})
	}
}

func TestFamilies(t *testing.T) {
	got := loadedFamilies()
	if len(got) != 14 {
		t.Fatalf("loadedFamilies() returned %d entries, want 14", len(got))
	}

	for i := 1; i < len(got); i++ {
		if got[i-1].family > got[i].family {
			t.Fatalf("loadedFamilies() not sorted by family: %q comes before %q", got[i-1].family, got[i].family)
		}
	}

	var bebasNeue, inter *fontFamily
	for i := range got {
		switch got[i].slug {
		case "bebas-neue":
			bebasNeue = &got[i]
		case "inter":
			inter = &got[i]
		default:
			// Not a family under test.
		}
	}

	if bebasNeue == nil {
		t.Fatal("loadedFamilies() missing bebas-neue")
	}
	if want := []int{400}; !reflect.DeepEqual(bebasNeue.weights, want) {
		t.Errorf("bebas-neue weights = %v, want %v", bebasNeue.weights, want)
	}

	if inter == nil {
		t.Fatal("loadedFamilies() missing inter")
	}
	if want := []int{400, 700}; !reflect.DeepEqual(inter.weights, want) {
		t.Errorf("inter weights = %v, want %v", inter.weights, want)
	}
}

func TestSearchFiltersBySource(t *testing.T) {
	// Query all families, but scope to just Inter by display name.
	results := searchFonts("", assetcore.Filter{Only: []string{knownFamily}}, 200)
	if len(results) != 1 || results[0].family != knownFamily {
		t.Fatalf("searchFonts scoped to %q = %+v, want a single Inter result", knownFamily, results)
	}

	// Excluding Inter must drop it.
	for _, m := range searchFonts("", assetcore.Filter{Except: []string{knownFamily}}, 200) {
		if m.family == knownFamily {
			t.Errorf("searchFonts with Inter excluded still returned it")
		}
	}
}

func TestSearchMapsFontHits(t *testing.T) {
	p := New()

	assets, err := p.Search(t.Context(), assetcore.SearchOpts{Query: knownSlug, Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	var inter *assetcore.Asset
	for i := range assets {
		if assets[i].Title == knownFamily {
			inter = &assets[i]
		}
	}
	if inter == nil {
		t.Fatalf("Search(%q) did not return Inter", knownSlug)
	}
	if inter.ID != assetcore.AssetID(providerName, knownSlug) {
		t.Errorf("Inter asset.ID = %q, want composite id", inter.ID)
	}
	if inter.Meta[assetcore.MetaCategory] != categorySans {
		t.Errorf("Inter category meta = %q, want %q", inter.Meta[assetcore.MetaCategory], categorySans)
	}
	if inter.Meta[assetcore.MetaWeights] != "400,700" {
		t.Errorf("Inter weights meta = %q, want 400,700", inter.Meta[assetcore.MetaWeights])
	}
}

func TestSourcesReportsFamiliesWithLicenseCountCategory(t *testing.T) {
	p := New()

	srcs := p.Sources()
	if len(srcs) != 14 {
		t.Fatalf("Sources() length = %d, want 14", len(srcs))
	}

	var inter *assetcore.Source
	for i := range srcs {
		if srcs[i].Name == knownFamily {
			inter = &srcs[i]
		}
	}
	if inter == nil {
		t.Fatal("Sources() missing Inter")
	}
	if inter.License.SPDX != "OFL-1.1" {
		t.Errorf("Inter license = %q, want OFL-1.1", inter.License.SPDX)
	}
	if inter.Count != 2 {
		t.Errorf("Inter variant count = %d, want 2", inter.Count)
	}
	if inter.Meta[assetcore.MetaCategory] != categorySans {
		t.Errorf("Inter source category = %q, want %q", inter.Meta[assetcore.MetaCategory], categorySans)
	}
}

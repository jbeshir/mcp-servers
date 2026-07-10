package embeddedfonts

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
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

func TestEveryFamilyHasLicense(t *testing.T) {
	c, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	_ = New(c)

	for _, fam := range loadedFamilies() {
		license, _, ok := c.FontLicense(fam.family)
		if !ok {
			t.Errorf("FontLicense(%q) ok = false, want true", fam.family)
		}
		if license == "" {
			t.Errorf("FontLicense(%q) license is empty", fam.family)
		}
	}
}

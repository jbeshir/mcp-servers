package fonts

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const (
	knownFamily   = "Inter"
	knownSlug     = "inter"
	knownFilename = "inter-latin-400-normal.woff2"
)

func TestGetKnownFamilyByDisplayName(t *testing.T) {
	f, err := Get(knownFamily, 400, "normal")
	if err != nil {
		t.Fatalf("Get(%q, 400, %q) returned unexpected error: %v", knownFamily, styleNormal, err)
	}
	if len(f.Data) == 0 {
		t.Fatalf("Get(%q, 400, %q) returned empty Data", knownFamily, styleNormal)
	}
	if f.Filename != knownFilename {
		t.Fatalf("Get(%q, 400, %q).Filename = %q, want %q", knownFamily, styleNormal, f.Filename, knownFilename)
	}
	if !bytes.HasPrefix(f.Data, []byte("wOF2")) {
		t.Fatalf("Get(%q, 400, %q).Data does not start with the woff2 magic bytes wOF2", knownFamily, styleNormal)
	}
}

func TestGetKnownFamilyBySlug(t *testing.T) {
	f, err := Get(knownSlug, 700, "normal")
	if err != nil {
		t.Fatalf("Get(%q, 700, %q) returned unexpected error: %v", knownSlug, styleNormal, err)
	}
	if len(f.Data) == 0 {
		t.Fatalf("Get(%q, 700, %q) returned empty Data", knownSlug, styleNormal)
	}
	if want := "inter-latin-700-normal.woff2"; f.Filename != want {
		t.Fatalf("Get(%q, 700, %q).Filename = %q, want %q", knownSlug, styleNormal, f.Filename, want)
	}
}

func TestFontFace(t *testing.T) {
	f, err := Get(knownFamily, 400, "normal")
	if err != nil {
		t.Fatalf("Get(%q, 400, %q) returned unexpected error: %v", knownFamily, styleNormal, err)
	}

	css := FontFace(knownFamily, f)
	for _, want := range []string{"@font-face", "font-weight: 400", knownFilename} {
		if !strings.Contains(css, want) {
			t.Errorf("FontFace(%q, f) = %q, want it to contain %q", knownFamily, css, want)
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
			_, err := Get(tt.family, tt.weight, tt.style)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get(%q, %d, %q) error = %v, want ErrNotFound", tt.family, tt.weight, tt.style, err)
			}
		})
	}
}

func TestFamilies(t *testing.T) {
	got := Families()
	if len(got) != 14 {
		t.Fatalf("Families() returned %d entries, want 14", len(got))
	}

	for i := 1; i < len(got); i++ {
		if got[i-1].Family > got[i].Family {
			t.Fatalf("Families() not sorted by Family: %q comes before %q", got[i-1].Family, got[i].Family)
		}
	}

	var bebasNeue, inter *Meta
	for i := range got {
		switch got[i].Slug {
		case "bebas-neue":
			bebasNeue = &got[i]
		case "inter":
			inter = &got[i]
		default:
			// Not a family under test.
		}
	}

	if bebasNeue == nil {
		t.Fatal("Families() missing bebas-neue")
	}
	if want := []int{400}; !reflect.DeepEqual(bebasNeue.Weights, want) {
		t.Errorf("bebas-neue Weights = %v, want %v", bebasNeue.Weights, want)
	}

	if inter == nil {
		t.Fatal("Families() missing inter")
	}
	if want := []int{400, 700}; !reflect.DeepEqual(inter.Weights, want) {
		t.Errorf("inter Weights = %v, want %v", inter.Weights, want)
	}
}

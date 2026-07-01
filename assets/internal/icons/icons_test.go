package icons

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderLucideArrow(t *testing.T) {
	svg, err := Render("lucide", "a-arrow-down", "", 0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	out := string(svg)
	if !strings.Contains(out, `viewBox="0 0 24 24"`) {
		t.Errorf("Render() output missing viewBox: %s", out)
	}
	if !strings.Contains(out, "<path") {
		t.Errorf("Render() output missing <path: %s", out)
	}
	if !strings.HasPrefix(out, "<svg ") || !strings.HasSuffix(out, "</svg>") {
		t.Errorf("Render() output not wrapped in <svg>...</svg>: %s", out)
	}
}

func TestRenderColorAndSize(t *testing.T) {
	svg, err := Render("lucide", "a-arrow-down", "#ff0000", 32)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	out := string(svg)
	for _, want := range []string{`width="32"`, `height="32"`, `viewBox="0 0 24 24"`, `color="#ff0000"`} {
		if !strings.Contains(out, want) {
			t.Errorf("Render() output missing %q: %s", want, out)
		}
	}
}

func TestRenderBootstrapIconsGrid(t *testing.T) {
	svg, err := Render("bootstrap-icons", "alarm", "", 0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	if !strings.Contains(string(svg), `viewBox="0 0 16 16"`) {
		t.Errorf("Render() output missing 16x16 viewBox: %s", svg)
	}
}

func TestRenderPhosphorGrid(t *testing.T) {
	svg, err := Render("phosphor", "acorn", "", 0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	if !strings.Contains(string(svg), `viewBox="0 0 256 256"`) {
		t.Errorf("Render() output missing 256x256 viewBox: %s", svg)
	}
}

func TestSearchLucideArrow(t *testing.T) {
	results := Search("arrow", "lucide", 10)
	if len(results) == 0 {
		t.Fatal("Search() returned no results, want at least one")
	}
	for _, m := range results {
		if m.Set != "lucide" {
			t.Errorf("Search() result Set = %q, want %q", m.Set, "lucide")
		}
	}
}

func TestRenderNotFound(t *testing.T) {
	if _, err := Render("lucide", "definitely-not-an-icon", "", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("Render() unknown icon error = %v, want ErrNotFound", err)
	}
	if _, err := Render("definitely-not-a-set", "a-arrow-down", "", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("Render() unknown set error = %v, want ErrNotFound", err)
	}
}

func TestSets(t *testing.T) {
	want := []string{
		"bootstrap-icons", "feather", "heroicons", "lucide",
		"material-symbols", "phosphor", "simple-icons", "tabler",
	}
	got := Sets()
	if len(got) != len(want) {
		t.Fatalf("Sets() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("Sets()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

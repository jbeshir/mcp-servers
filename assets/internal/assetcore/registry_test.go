package assetcore

import (
	"context"
	"testing"
)

// embeddedIconsName is a sample provider name reused across these tests.
const embeddedIconsName = "embedded-icons"

// fakeIconProvider is a minimal IconProvider used across the assetcore tests. A non-nil err makes
// both Search and Fetch fail; otherwise Search returns page and Fetch echoes the provider name.
type fakeIconProvider struct {
	name string
	page Page
	err  error
}

func (f fakeIconProvider) Name() string { return f.name }
func (f fakeIconProvider) Kind() Kind   { return KindIcon }

func (f fakeIconProvider) Search(_ context.Context, _ IconQuery) (Page, error) {
	if f.err != nil {
		return Page{}, f.err
	}

	return f.page, nil
}

func (f fakeIconProvider) Fetch(_ context.Context, a Asset) (Blob, error) {
	if f.err != nil {
		return Blob{}, f.err
	}

	return Blob{Asset: a, Content: []byte(f.name)}, nil
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
	r.AddIcon(fakeIconProvider{name: "dup", page: Page{Total: 1}})
	r.AddIcon(fakeIconProvider{name: "dup", page: Page{Total: 2}})

	got := r.Icons()
	if len(got) != 1 {
		t.Fatalf("Icons() length = %d, want 1", len(got))
	}

	page, _ := got[0].Search(t.Context(), IconQuery{})
	if page.Total != 2 {
		t.Errorf("second registration did not win: Total = %d, want 2", page.Total)
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

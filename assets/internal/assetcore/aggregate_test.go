package assetcore

import (
	"errors"
	"testing"
)

func TestSearchIconsMergesAndDedupes(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a", page: Page{Assets: []Asset{
		{Source: "s", ID: "1"}, {Source: "s", ID: "2"},
	}}})
	r.AddIcon(fakeIconProvider{name: "b", page: Page{Assets: []Asset{
		{Source: "s", ID: "2"}, {Source: "s", ID: "3"},
	}}})

	page, warns := r.SearchIcons(t.Context(), IconQuery{})

	if len(warns) != 0 {
		t.Fatalf("warnings = %v, want none", warns)
	}

	var ids []string
	for _, a := range page.Assets {
		ids = append(ids, a.ID)
	}
	// (s,2) is emitted by both providers but must appear once, first-provider-wins.
	want := []string{"1", "2", "3"}
	if len(ids) != len(want) {
		t.Fatalf("merged ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("merged ids[%d] = %q, want %q", i, ids[i], id)
		}
	}
}

func TestSearchIconsDegradesFailingProvider(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "good", page: Page{Assets: []Asset{{Source: "s", ID: "1"}}}})
	r.AddIcon(fakeIconProvider{name: "bad", err: errors.New("boom")})

	page, warns := r.SearchIcons(t.Context(), IconQuery{})

	// The failing provider must not fail the whole search: the good provider's result survives.
	if len(page.Assets) != 1 || page.Assets[0].ID != "1" {
		t.Fatalf("assets = %+v, want the single good result", page.Assets)
	}

	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warns)
	}
	if warns[0].Provider != "bad" {
		t.Errorf("warning provider = %q, want %q", warns[0].Provider, "bad")
	}
	if warns[0].Err != "boom" {
		t.Errorf("warning err = %q, want %q", warns[0].Err, "boom")
	}
}

func TestFetchIconTriesUntilFound(t *testing.T) {
	r := NewRegistry()
	// Sorted by name: "a-miss" is tried first and reports ErrNotFound, then "b-hit" succeeds.
	r.AddIcon(fakeIconProvider{name: "a-miss", err: ErrNotFound})
	r.AddIcon(fakeIconProvider{name: "b-hit"})

	blob, err := r.FetchIcon(t.Context(), Asset{})
	if err != nil {
		t.Fatalf("FetchIcon error = %v, want nil", err)
	}
	if string(blob.Content) != "b-hit" {
		t.Errorf("FetchIcon served %q, want the b-hit provider", string(blob.Content))
	}
}

func TestFetchIconAllMissReturnsNotFound(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "x", err: ErrNotFound})

	if _, err := r.FetchIcon(t.Context(), Asset{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIcon error = %v, want ErrNotFound", err)
	}
}

func TestFetchIconPropagatesRealError(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "x", err: errors.New("disk full")})

	_, err := r.FetchIcon(t.Context(), Asset{})
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIcon error = %v, want a non-ErrNotFound error", err)
	}
}

func TestFetchIconEmptyRegistryReturnsNotFound(t *testing.T) {
	r := NewRegistry()

	if _, err := r.FetchIcon(t.Context(), Asset{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchIcon error = %v, want ErrNotFound", err)
	}
}

package assetcore

import (
	"errors"
	"testing"
)

func TestSearchIconsMergesAndDedupesByLogicalIdentity(t *testing.T) {
	// The same logical asset (Source=s, Title=camera) is served by both providers with a different
	// composite ID each; merge must dedup on (Source, Title), first-provider-wins.
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a", assets: []Asset{
		{Source: "s", Title: "camera", ID: "a:s/camera"},
		{Source: "s", Title: "home", ID: "a:s/home"},
	}})
	r.AddIcon(fakeIconProvider{name: "b", assets: []Asset{
		{Source: "s", Title: "camera", ID: "b:s/camera"},
		{Source: "s", Title: "gear", ID: "b:s/gear"},
	}})

	assets, warns := r.SearchIcons(t.Context(), SearchOpts{})
	if len(warns) != 0 {
		t.Fatalf("warnings = %v, want none", warns)
	}

	var titles, ids []string
	for _, a := range assets {
		titles = append(titles, a.Title)
		ids = append(ids, a.ID)
	}

	wantTitles := []string{"camera", "home", "gear"}
	if len(titles) != len(wantTitles) {
		t.Fatalf("merged titles = %v, want %v", titles, wantTitles)
	}
	for i, want := range wantTitles {
		if titles[i] != want {
			t.Errorf("merged titles[%d] = %q, want %q", i, titles[i], want)
		}
	}
	// The winning "camera" must be provider a's (first-provider-wins), keeping its composite ID.
	if ids[0] != "a:s/camera" {
		t.Errorf("deduped camera ID = %q, want a:s/camera (first provider wins)", ids[0])
	}
}

func TestSearchIconsDegradesFailingProvider(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "good", assets: []Asset{{Source: "s", Title: "t", ID: "good:s/t"}}})
	r.AddIcon(fakeIconProvider{name: "bad", err: errors.New("boom")})

	assets, warns := r.SearchIcons(t.Context(), SearchOpts{})

	if len(assets) != 1 || assets[0].ID != "good:s/t" {
		t.Fatalf("assets = %+v, want the single good result", assets)
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

func TestSearchIconsProviderFilterSkipsBeforeFanOut(t *testing.T) {
	// The excluded provider errors; if it were searched it would produce a warning. The Providers
	// filter must skip it entirely, so no warning and only the allowed provider's assets appear.
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "keep", assets: []Asset{{Source: "s", Title: "t", ID: "keep:s/t"}}})
	r.AddIcon(fakeIconProvider{name: "drop", err: errors.New("should never run")})

	assets, warns := r.SearchIcons(t.Context(), SearchOpts{Providers: Filter{Except: []string{"drop"}}})

	if len(warns) != 0 {
		t.Fatalf("warnings = %v, want none (excluded provider must be skipped, not run)", warns)
	}
	if len(assets) != 1 || assets[0].ID != "keep:s/t" {
		t.Fatalf("assets = %+v, want only the kept provider's result", assets)
	}
}

func TestSearchIconsProviderOnlyFilter(t *testing.T) {
	r := NewRegistry()
	r.AddIcon(fakeIconProvider{name: "a", assets: []Asset{{Source: "s", Title: "ta", ID: "a:s/ta"}}})
	r.AddIcon(fakeIconProvider{name: "b", assets: []Asset{{Source: "s", Title: "tb", ID: "b:s/tb"}}})

	assets, _ := r.SearchIcons(t.Context(), SearchOpts{Providers: Filter{Only: []string{"b"}}})

	if len(assets) != 1 || assets[0].ID != "b:s/tb" {
		t.Fatalf("assets = %+v, want only provider b's result", assets)
	}
}

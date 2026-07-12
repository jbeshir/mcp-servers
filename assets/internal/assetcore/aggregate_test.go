package assetcore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

func TestSearchIconsMergesAndDedupesByLogicalIdentity(t *testing.T) {
	// The same logical asset (Source=s, Title=camera) is served by both providers with a different
	// composite ID each; merge must dedup on (Source, Title), first-provider-wins.
	r := assetcore.NewRegistry()

	a := newIconProvider(t, "a")
	a.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{Assets: []assetcore.Asset{
		{Source: "s", Title: "camera", ID: "a:s/camera"},
		{Source: "s", Title: "home", ID: "a:s/home"},
	}}, nil)
	r.AddIcon(a)

	b := newIconProvider(t, "b")
	b.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{Assets: []assetcore.Asset{
		{Source: "s", Title: "camera", ID: "b:s/camera"},
		{Source: "s", Title: "gear", ID: "b:s/gear"},
	}}, nil)
	r.AddIcon(b)

	assets, _, warns := r.SearchIcons(t.Context(), assetcore.SearchOpts{})
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
	r := assetcore.NewRegistry()

	good := newIconProvider(t, "good")
	good.EXPECT().Search(mock.Anything, mock.Anything).
		Return(assetcore.SearchResult{Assets: []assetcore.Asset{{Source: "s", Title: "t", ID: "good:s/t"}}}, nil)
	r.AddIcon(good)

	bad := newIconProvider(t, "bad")
	bad.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{}, errors.New("boom"))
	r.AddIcon(bad)

	assets, _, warns := r.SearchIcons(t.Context(), assetcore.SearchOpts{})

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
	// The excluded provider would produce a warning if searched. The Providers filter must skip it
	// entirely: no Search expectation is set on it, so mockery fails the test if it is ever searched.
	r := assetcore.NewRegistry()

	keep := newIconProvider(t, "keep")
	keep.EXPECT().Search(mock.Anything, mock.Anything).
		Return(assetcore.SearchResult{Assets: []assetcore.Asset{{Source: "s", Title: "t", ID: "keep:s/t"}}}, nil)
	r.AddIcon(keep)

	r.AddIcon(newIconProvider(t, "drop"))

	assets, _, warns := r.SearchIcons(
		t.Context(), assetcore.SearchOpts{Providers: assetcore.Filter{Except: []string{"drop"}}},
	)

	if len(warns) != 0 {
		t.Fatalf("warnings = %v, want none (excluded provider must be skipped, not run)", warns)
	}
	if len(assets) != 1 || assets[0].ID != "keep:s/t" {
		t.Fatalf("assets = %+v, want only the kept provider's result", assets)
	}
}

func TestSearchIconsProviderOnlyFilter(t *testing.T) {
	r := assetcore.NewRegistry()

	// a is not in the Only list, so it must be skipped before fan-out — no Search expectation.
	r.AddIcon(newIconProvider(t, "a"))

	b := newIconProvider(t, "b")
	b.EXPECT().Search(mock.Anything, mock.Anything).
		Return(assetcore.SearchResult{Assets: []assetcore.Asset{{Source: "s", Title: "tb", ID: "b:s/tb"}}}, nil)
	r.AddIcon(b)

	assets, _, _ := r.SearchIcons(t.Context(), assetcore.SearchOpts{Providers: assetcore.Filter{Only: []string{"b"}}})

	if len(assets) != 1 || assets[0].ID != "b:s/tb" {
		t.Fatalf("assets = %+v, want only provider b's result", assets)
	}
}

func TestSearchIconsCursorRoundTripsAndNarrowsToPagingProvider(t *testing.T) {
	// pager has a second page; other is exhausted after its first. The aggregate cursor returned from
	// page 1 must, when passed back in, query only pager (the provider that still has a next page).
	r := assetcore.NewRegistry()

	pager := newIconProvider(t, "pager")
	pager.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
			if opts.Cursor == "" {
				return assetcore.SearchResult{
					Assets:     []assetcore.Asset{{Source: "s", Title: "page1", ID: "pager:s/page1"}},
					NextCursor: "p2",
				}, nil
			}
			if opts.Cursor != "p2" {
				t.Fatalf("pager.Search got Cursor = %q, want \"p2\"", opts.Cursor)
			}

			return assetcore.SearchResult{
				Assets: []assetcore.Asset{{Source: "s", Title: "page2", ID: "pager:s/page2"}},
			}, nil
		},
	)
	r.AddIcon(pager)

	other := newIconProvider(t, "other")
	other.EXPECT().Search(mock.Anything, mock.Anything).
		Return(assetcore.SearchResult{Assets: []assetcore.Asset{{Source: "s", Title: "other", ID: "other:s/other"}}}, nil)
	r.AddIcon(other)

	assets1, cursor1, warns1 := r.SearchIcons(t.Context(), assetcore.SearchOpts{})
	if len(warns1) != 0 {
		t.Fatalf("page 1 warnings = %v, want none", warns1)
	}
	if cursor1 == "" {
		t.Fatal("page 1 next cursor is empty, want a token carrying pager's page 2 cursor")
	}
	if len(assets1) != 2 {
		t.Fatalf("page 1 assets = %+v, want one hit per provider", assets1)
	}

	assets2, cursor2, warns2 := r.SearchIcons(t.Context(), assetcore.SearchOpts{Cursor: cursor1})
	if len(warns2) != 0 {
		t.Fatalf("page 2 warnings = %v, want none", warns2)
	}
	if cursor2 != "" {
		t.Errorf("page 2 next cursor = %q, want \"\" (pager exhausted)", cursor2)
	}
	// other has no Search expectation covering a second call, so a mock failure here means the aggregate
	// queried a provider outside the decoded cursor's keys.
	if len(assets2) != 1 || assets2[0].ID != "pager:s/page2" {
		t.Fatalf("page 2 assets = %+v, want only pager's page 2 result", assets2)
	}
}

func TestSearchIconsRecoversPanickingProvider(t *testing.T) {
	r := assetcore.NewRegistry()

	good := newIconProvider(t, "good")
	good.EXPECT().Search(mock.Anything, mock.Anything).
		Return(assetcore.SearchResult{Assets: []assetcore.Asset{{Source: "s", Title: "t", ID: "good:s/t"}}}, nil)
	r.AddIcon(good)

	panicky := newIconProvider(t, "panicky")
	panicky.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, assetcore.SearchOpts) (assetcore.SearchResult, error) {
			panic("boom")
		},
	)
	r.AddIcon(panicky)

	assets, _, warns := r.SearchIcons(t.Context(), assetcore.SearchOpts{})

	if len(assets) != 1 || assets[0].ID != "good:s/t" {
		t.Fatalf("assets = %+v, want the single good result", assets)
	}
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warns)
	}
	if warns[0].Provider != "panicky" {
		t.Errorf("warning provider = %q, want %q", warns[0].Provider, "panicky")
	}
}

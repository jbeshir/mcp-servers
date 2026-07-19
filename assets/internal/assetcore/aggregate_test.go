package assetcore_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

func TestSearchIconsPreservesDistinctIDsWithSameTitle(t *testing.T) {
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

	wantTitles := []string{"camera", "home", "camera", "gear"}
	if len(titles) != len(wantTitles) {
		t.Fatalf("merged titles = %v, want %v", titles, wantTitles)
	}
	for i, want := range wantTitles {
		if titles[i] != want {
			t.Errorf("merged titles[%d] = %q, want %q", i, titles[i], want)
		}
	}
	if ids[0] != "a:s/camera" || ids[2] != "b:s/camera" {
		t.Errorf("same-title IDs = %v, want both distinct assets", ids)
	}
}

func TestSearchSpritesPreservesSamePackSameTitleVariants(t *testing.T) {
	r := assetcore.NewRegistry()
	p := newSpriteProvider(t, "assetsdb")
	p.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{Assets: []assetcore.Asset{
		{Source: "kenney", Title: "Tree", ID: "assetsdb:kenney/tree-a.png"},
		{Source: "kenney", Title: "Tree", ID: "assetsdb:kenney/tree-b.png"},
	}}, nil)
	r.AddSprite(p)
	assets, _, warns := r.SearchSprites(t.Context(), assetcore.SearchOpts{})
	if len(warns) != 0 || len(assets) != 2 {
		t.Fatalf("assets=%+v warnings=%v, want both same-title variants", assets, warns)
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

// retryExhaustingSearch is provider a's mock: it pages once (returning cursor "a2") then exhausts, and
// fails the test if queried a third time — proving an exhausted provider is dropped from the cursor.
func retryExhaustingSearch(t *testing.T, calls *int32) func(context.Context, assetcore.SearchOpts) (assetcore.SearchResult, error) {
	return func(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
		switch n := atomic.AddInt32(calls, 1); n {
		case 1:
			if opts.Cursor != "" {
				t.Errorf("a page 1 cursor = %q, want \"\"", opts.Cursor)
			}

			return assetcore.SearchResult{
				Assets:     []assetcore.Asset{{Source: "s", Title: "a1", ID: "a:s/a1"}},
				NextCursor: "a2",
			}, nil
		case 2:
			if opts.Cursor != "a2" {
				t.Errorf("a page 2 cursor = %q, want \"a2\"", opts.Cursor)
			}

			return assetcore.SearchResult{
				Assets: []assetcore.Asset{{Source: "s", Title: "a2", ID: "a:s/a2"}},
			}, nil
		default:
			t.Fatalf("a.Search called %d times, want 2 (an exhausted provider must not be re-queried)", n)

			return assetcore.SearchResult{}, nil
		}
	}
}

// retryErroringSearch is provider b's mock: it pages once (cursor "b2"), errors mid-pagination on page
// 2, then on page 3 asserts it was retried from the carried cursor "b2" rather than restarted.
func retryErroringSearch(t *testing.T, calls *int32) func(context.Context, assetcore.SearchOpts) (assetcore.SearchResult, error) {
	return func(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
		switch n := atomic.AddInt32(calls, 1); n {
		case 1:
			return assetcore.SearchResult{
				Assets:     []assetcore.Asset{{Source: "s", Title: "b1", ID: "b:s/b1"}},
				NextCursor: "b2",
			}, nil
		case 2:
			if opts.Cursor != "b2" {
				t.Errorf("b page 2 cursor = %q, want \"b2\"", opts.Cursor)
			}

			return assetcore.SearchResult{}, errors.New("blip")
		case 3:
			// The retry must resume from b's carried incoming cursor, not restart at page 1.
			if opts.Cursor != "b2" {
				t.Errorf("b retry cursor = %q, want \"b2\" (carried forward from the failed page)", opts.Cursor)
			}

			return assetcore.SearchResult{
				Assets: []assetcore.Asset{{Source: "s", Title: "b3", ID: "b:s/b3"}},
			}, nil
		default:
			t.Fatalf("b.Search called %d times, want 3", n)

			return assetcore.SearchResult{}, nil
		}
	}
}

func TestSearchIconsRetriesProviderErroringMidPagination(t *testing.T) {
	// Both a and b page past page 1. On page 2 a exhausts cleanly while b errors mid-pagination; b's
	// incoming cursor must be carried forward so page 3 retries it from where it was, whereas a
	// (exhausted, so absent from the cursor) is not queried again.
	r := assetcore.NewRegistry()

	var aCalls, bCalls int32

	a := newIconProvider(t, "a")
	a.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(retryExhaustingSearch(t, &aCalls))
	r.AddIcon(a)

	b := newIconProvider(t, "b")
	b.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(retryErroringSearch(t, &bCalls))
	r.AddIcon(b)

	// Page 1: both providers page on, so the cursor keys both.
	_, cursor1, warns1 := r.SearchIcons(t.Context(), assetcore.SearchOpts{})
	if len(warns1) != 0 {
		t.Fatalf("page 1 warnings = %v, want none", warns1)
	}
	if cursor1 == "" {
		t.Fatal("page 1 cursor is empty, want a token carrying both providers")
	}

	// Page 2: a exhausts, b errors mid-pagination and must be carried forward for retry.
	_, cursor2, warns2 := r.SearchIcons(t.Context(), assetcore.SearchOpts{Cursor: cursor1})
	if len(warns2) != 1 {
		t.Fatalf("page 2 warnings = %v, want exactly one (b's blip)", warns2)
	}
	if warns2[0].Provider != "b" {
		t.Errorf("page 2 warning provider = %q, want %q", warns2[0].Provider, "b")
	}
	if warns2[0].Err != "blip" {
		t.Errorf("page 2 warning err = %q, want %q", warns2[0].Err, "blip")
	}
	if cursor2 == "" {
		t.Fatal("page 2 cursor is empty, want b carried forward for retry")
	}

	// Page 3: only b is retried (a exhausted); b's mock asserts the carried cursor, and a's call count
	// proves the exhausted provider was not queried again.
	assets3, _, warns3 := r.SearchIcons(t.Context(), assetcore.SearchOpts{Cursor: cursor2})
	if len(warns3) != 0 {
		t.Fatalf("page 3 warnings = %v, want none", warns3)
	}
	if len(assets3) != 1 || assets3[0].ID != "b:s/b3" {
		t.Fatalf("page 3 assets = %+v, want only b's retried page", assets3)
	}
	if got := atomic.LoadInt32(&aCalls); got != 2 {
		t.Errorf("a.Search calls = %d, want 2 (exhausted provider must not be re-queried on page 3)", got)
	}
}

func TestSearchIconsFirstPageErrorDropsProvider(t *testing.T) {
	// A provider that fails on its first page (empty incoming cursor) must NOT be encoded into the
	// returned cursor: a fresh search re-queries every provider, so nothing is lost by dropping it, and
	// this avoids a permanently-non-empty cursor for a provider that never paginated.
	r := assetcore.NewRegistry()

	good := newIconProvider(t, "good")
	good.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
			switch opts.Cursor {
			case "":
				return assetcore.SearchResult{
					Assets:     []assetcore.Asset{{Source: "s", Title: "g1", ID: "good:s/g1"}},
					NextCursor: "g2",
				}, nil
			case "g2":
				return assetcore.SearchResult{
					Assets: []assetcore.Asset{{Source: "s", Title: "g2", ID: "good:s/g2"}},
				}, nil
			default:
				t.Fatalf("good.Search unexpected cursor %q", opts.Cursor)

				return assetcore.SearchResult{}, nil
			}
		},
	)
	r.AddIcon(good)

	// bad fails only on the first page; its single Once() expectation makes a second query (which would
	// happen if it were wrongly encoded into the cursor) fail the mock.
	bad := newIconProvider(t, "bad")
	bad.EXPECT().Search(mock.Anything, mock.Anything).Return(assetcore.SearchResult{}, errors.New("boom")).Once()
	r.AddIcon(bad)

	_, cursor1, warns1 := r.SearchIcons(t.Context(), assetcore.SearchOpts{})
	if len(warns1) != 1 || warns1[0].Provider != "bad" {
		t.Fatalf("page 1 warnings = %v, want exactly one naming bad", warns1)
	}
	if cursor1 == "" {
		t.Fatal("page 1 cursor is empty, want good's next page carried forward")
	}

	// Page 2 must query only good; if bad had been encoded into cursor1 it would be queried again and
	// its Once() expectation would fail.
	assets2, _, warns2 := r.SearchIcons(t.Context(), assetcore.SearchOpts{Cursor: cursor1})
	if len(warns2) != 0 {
		t.Fatalf("page 2 warnings = %v, want none (a first-page failure must not be retried mid-session)", warns2)
	}
	if len(assets2) != 1 || assets2[0].ID != "good:s/g2" {
		t.Fatalf("page 2 assets = %+v, want only good's second page", assets2)
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

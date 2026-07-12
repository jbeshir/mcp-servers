package assetcore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// searchProviderTimeout bounds each provider's Search during a fan-out so one slow provider cannot
// stall the aggregate. Embedded providers are in-process and never approach it; it is set generously
// for the remote providers a later PR will add, which can legitimately be slow.
const searchProviderTimeout = 30 * time.Second

// cursorProvider names the pseudo-provider a Warning is attributed to when the aggregate cursor
// itself, rather than any single provider, is at fault.
const cursorProvider = "cursor"

// Warning records a single provider degrading during an aggregate search: its results are dropped
// but the search as a whole still returns, mirroring Openverse's warnings[] envelope.
type Warning struct {
	Provider string
	Err      string
}

// SearchIcons fans out across the icon providers named in opts.Cursor (or, on a first page, all
// providers allowed by opts.Providers) and merges the results.
func (r *Registry) SearchIcons(ctx context.Context, opts SearchOpts) ([]Asset, string, []Warning) {
	return aggregateSearch(ctx, r.Icons(), opts,
		func(c context.Context, p IconProvider, o SearchOpts) (SearchResult, error) {
			return p.Search(c, o)
		})
}

// SearchIllustrations fans out across the illustration providers named in opts.Cursor (or, on a first
// page, all providers allowed by opts.Providers) and merges the results.
func (r *Registry) SearchIllustrations(ctx context.Context, opts SearchOpts) ([]Asset, string, []Warning) {
	return aggregateSearch(ctx, r.Illustrations(), opts,
		func(c context.Context, p IllustrationProvider, o SearchOpts) (SearchResult, error) {
			return p.Search(c, o)
		})
}

// SearchFonts fans out across the font providers named in opts.Cursor (or, on a first page, all
// providers allowed by opts.Providers) and merges the results.
func (r *Registry) SearchFonts(ctx context.Context, opts SearchOpts) ([]Asset, string, []Warning) {
	return aggregateSearch(ctx, r.Fonts(), opts,
		func(c context.Context, p FontProvider, o SearchOpts) (SearchResult, error) {
			return p.Search(c, o)
		})
}

// SearchPhotos fans out across the photo providers named in opts.Cursor (or, on a first page, all
// providers allowed by opts.Providers) and merges the results.
func (r *Registry) SearchPhotos(ctx context.Context, opts SearchOpts) ([]Asset, string, []Warning) {
	return aggregateSearch(ctx, r.Photos(), opts,
		func(c context.Context, p PhotoProvider, o SearchOpts) (SearchResult, error) {
			return p.Search(c, o)
		})
}

// SearchTextures fans out across the texture providers named in opts.Cursor (or, on a first page, all
// providers allowed by opts.Providers) and merges the results.
func (r *Registry) SearchTextures(ctx context.Context, opts SearchOpts) ([]Asset, string, []Warning) {
	return aggregateSearch(ctx, r.Textures(), opts,
		func(c context.Context, p TextureProvider, o SearchOpts) (SearchResult, error) {
			return p.Search(c, o)
		})
}

// allowedProviders returns the subset of provs whose Name the filter allows, preserving order. An
// all-allowing filter returns provs unchanged.
func allowedProviders[P Provider](provs []P, f Filter) []P {
	if len(f.Only) == 0 && len(f.Except) == 0 {
		return provs
	}

	out := make([]P, 0, len(provs))
	for _, p := range provs {
		if f.Allows(p.Name()) {
			out = append(out, p)
		}
	}

	return out
}

// aggregateSearch runs search concurrently across the providers targeted by opts.Cursor, merges their
// hits, and encodes their NextCursor values into the single opaque cursor a caller passes back for the
// next page. On a first page (opts.Cursor == "") every provider allowed by opts.Providers is queried;
// on a continuation, only the providers named as keys in the decoded cursor are queried, each with its
// own per-provider cursor restored into a copy of opts. A malformed cursor is reported as a Warning
// with an empty result rather than an error, matching the degrade-not-fail shape of a provider fault.
//
// Cross-page dedup is best-effort only: merge dedups within a single aggregateSearch call by (Source,
// Title), but nothing tracks identities already surfaced on an earlier page, so a logical asset that
// shifts between two providers' pages between calls could in principle reappear. This is acceptable for
// the in-process, single-page embedded providers and is revisited if a remote provider's pagination
// makes it a real concern.
func aggregateSearch[P Provider](
	ctx context.Context,
	all []P,
	opts SearchOpts,
	search func(context.Context, P, SearchOpts) (SearchResult, error),
) ([]Asset, string, []Warning) {
	prevCursors, err := decodeCursor(opts.Cursor)
	if err != nil {
		return nil, "", []Warning{{Provider: cursorProvider, Err: fmt.Sprintf("invalid cursor: %v", err)}}
	}

	targets := targetProviders(all, opts.Providers, prevCursors)

	results := make([][]Asset, len(targets))
	nextCursors := make(map[string]string)
	warns := make([]Warning, 0)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for i, p := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()

			providerOpts := opts
			providerOpts.Cursor = prevCursors[p.Name()]

			cctx, cancel := context.WithTimeout(ctx, searchProviderTimeout)
			defer cancel()

			res, err := recoveringSearch(cctx, p, providerOpts, search)
			if err != nil {
				mu.Lock()
				warns = append(warns, Warning{Provider: p.Name(), Err: err.Error()})
				mu.Unlock()

				return
			}

			results[i] = res.Assets
			if res.NextCursor != "" {
				mu.Lock()
				nextCursors[p.Name()] = res.NextCursor
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	return merge(results), encodeCursor(nextCursors), warns
}

// targetProviders selects which providers an aggregateSearch call queries: every provider allowed by f
// on a first page (empty prevCursors), or only those named as keys in prevCursors on a continuation.
func targetProviders[P Provider](all []P, f Filter, prevCursors map[string]string) []P {
	if len(prevCursors) == 0 {
		return allowedProviders(all, f)
	}

	targets := make([]P, 0, len(prevCursors))
	for _, p := range all {
		if _, ok := prevCursors[p.Name()]; ok {
			targets = append(targets, p)
		}
	}

	return targets
}

// recoveringSearch calls search and converts a panic inside it into an error, so one misbehaving
// provider cannot crash the whole fan-out; the caller turns the error into a Warning like any other
// provider failure.
func recoveringSearch[P Provider](
	ctx context.Context,
	p P,
	opts SearchOpts,
	search func(context.Context, P, SearchOpts) (SearchResult, error),
) (res SearchResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return search(ctx, p, opts)
}

// merge concatenates lists in provider order, deduping by logical identity (Source, Title) so the
// same logical asset served by two providers appears once, first-provider-wins. Deduping on ID would
// not compose across providers because the composite ID embeds the provider name, so the same logical
// asset carries a different ID per provider.
func merge(lists [][]Asset) []Asset {
	seen := make(map[string]bool)
	var out []Asset

	for _, list := range lists {
		for _, a := range list {
			key := a.Source + "\x00" + a.Title
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, a)
		}
	}

	return out
}

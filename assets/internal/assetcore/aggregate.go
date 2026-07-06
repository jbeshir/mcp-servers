package assetcore

import (
	"context"
	"errors"
	"sync"
	"time"
)

// searchProviderTimeout bounds each provider's Search during a fan-out so one slow provider cannot
// stall the aggregate. Embedded providers are in-process and never approach it; it exists for the
// remote providers a later PR will add.
const searchProviderTimeout = 4 * time.Second

// Warning records a single provider degrading during an aggregate search: its results are dropped
// but the search as a whole still returns, mirroring Openverse's warnings[] envelope.
type Warning struct {
	Provider string
	Err      string
}

// SearchIcons fans out across every registered icon provider and merges the results.
func (r *Registry) SearchIcons(ctx context.Context, q IconQuery) (Page, []Warning) {
	return fanOutSearch(ctx, r.Icons(), func(c context.Context, p IconProvider) (Page, error) {
		return p.Search(c, q)
	})
}

// SearchIllustrations fans out across every registered illustration provider and merges the results.
func (r *Registry) SearchIllustrations(ctx context.Context, q IllustrationQuery) (Page, []Warning) {
	return fanOutSearch(ctx, r.Illustrations(), func(c context.Context, p IllustrationProvider) (Page, error) {
		return p.Search(c, q)
	})
}

// SearchFonts fans out across every registered font provider and merges the results.
func (r *Registry) SearchFonts(ctx context.Context, q FontQuery) (Page, []Warning) {
	return fanOutSearch(ctx, r.Fonts(), func(c context.Context, p FontProvider) (Page, error) {
		return p.Search(c, q)
	})
}

// FetchIcon fetches an icon from the first registered provider that has it.
func (r *Registry) FetchIcon(ctx context.Context, a Asset) (Blob, error) {
	_, b, err := fetchFirst(ctx, r.Icons(), a)
	return b, err
}

// FetchIllustration fetches an illustration from the first registered provider that has it.
func (r *Registry) FetchIllustration(ctx context.Context, a Asset) (Blob, error) {
	_, b, err := fetchFirst(ctx, r.Illustrations(), a)
	return b, err
}

// FetchFont fetches a font from the first registered provider that has it, returning that provider so
// the caller can type-assert an optional FontFaceRenderer for @font-face CSS.
func (r *Registry) FetchFont(ctx context.Context, a Asset) (Blob, FontProvider, error) {
	p, b, err := fetchFirst(ctx, r.Fonts(), a)
	return b, p, err
}

// fanOutSearch runs search concurrently across provs with a per-provider timeout, collecting a
// Warning for each provider that errors instead of failing the whole search, then merges the
// surviving pages in provider order. It is generic over the per-kind provider types so the three
// SearchX methods share one implementation.
func fanOutSearch[P Provider](
	ctx context.Context,
	provs []P,
	search func(context.Context, P) (Page, error),
) (Page, []Warning) {
	pages := make([]Page, len(provs))
	warns := make([]Warning, 0)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for i, p := range provs {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cctx, cancel := context.WithTimeout(ctx, searchProviderTimeout)
			defer cancel()

			pg, err := search(cctx, p)
			if err != nil {
				mu.Lock()
				warns = append(warns, Warning{Provider: p.Name(), Err: err.Error()})
				mu.Unlock()

				return
			}

			pages[i] = pg
		}()
	}

	wg.Wait()

	return merge(pages), warns
}

// merge concatenates pages in provider order, dropping later duplicates keyed by (Source, ID) so the
// first provider to emit an asset wins. Cursor/Total are per-provider concepts that do not compose
// across providers, so the merged Page reports only the deduped assets.
func merge(pages []Page) Page {
	seen := make(map[string]bool)
	out := Page{Total: -1}

	for _, pg := range pages {
		for _, a := range pg.Assets {
			key := a.Source + "\x00" + a.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			out.Assets = append(out.Assets, a)
		}
	}

	return out
}

// fetchProvider is the constraint for fetchFirst: any provider that can materialize an Asset. Each
// per-kind provider interface (IconProvider, IllustrationProvider, FontProvider) satisfies it.
type fetchProvider interface {
	Provider
	Fetch(ctx context.Context, a Asset) (Blob, error)
}

// fetchFirst tries each provider's Fetch in order, skipping providers that report ErrNotFound and
// returning the first success (with the serving provider) or a non-ErrNotFound error. If no provider
// has the asset it returns ErrNotFound. Generic so the three FetchX helpers share one implementation.
func fetchFirst[P fetchProvider](ctx context.Context, provs []P, a Asset) (P, Blob, error) {
	var zero P

	for _, p := range provs {
		blob, err := p.Fetch(ctx, a)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}

			return zero, Blob{}, err
		}

		return p, blob, nil
	}

	return zero, Blob{}, ErrNotFound
}

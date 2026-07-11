package assetcore

import (
	"context"
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

// SearchIcons fans out across the icon providers allowed by opts.Providers and merges the results.
func (r *Registry) SearchIcons(ctx context.Context, opts SearchOpts) (Page, []Warning) {
	provs := allowedProviders(r.Icons(), opts.Providers)

	return fanOutSearch(ctx, provs, func(c context.Context, p IconProvider) (Page, error) {
		return p.Search(c, opts)
	})
}

// SearchIllustrations fans out across the illustration providers allowed by opts.Providers and merges
// the results.
func (r *Registry) SearchIllustrations(ctx context.Context, opts SearchOpts) (Page, []Warning) {
	provs := allowedProviders(r.Illustrations(), opts.Providers)

	return fanOutSearch(ctx, provs, func(c context.Context, p IllustrationProvider) (Page, error) {
		return p.Search(c, opts)
	})
}

// SearchFonts fans out across the font providers allowed by opts.Providers and merges the results.
func (r *Registry) SearchFonts(ctx context.Context, opts SearchOpts) (Page, []Warning) {
	provs := allowedProviders(r.Fonts(), opts.Providers)

	return fanOutSearch(ctx, provs, func(c context.Context, p FontProvider) (Page, error) {
		return p.Search(c, opts)
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

// merge concatenates pages in provider order, deduping by logical identity (Source, Title) so the
// same logical asset served by two providers appears once, first-provider-wins. Deduping on ID would
// not compose across providers because the composite ID embeds the provider name, so the same logical
// asset carries a different ID per provider. Cursor/Total are per-provider concepts that do not
// compose, so the merged Page reports only the deduped assets.
func merge(pages []Page) Page {
	seen := make(map[string]bool)
	out := Page{Total: -1}

	for _, pg := range pages {
		for _, a := range pg.Assets {
			key := a.Source + "\x00" + a.Title
			if seen[key] {
				continue
			}
			seen[key] = true
			out.Assets = append(out.Assets, a)
		}
	}

	return out
}

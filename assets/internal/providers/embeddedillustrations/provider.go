// Package embeddedillustrations serves the vendored SVG illustration collections as an
// assetcore.IllustrationProvider. It owns its per-collection license metadata (folded from the former
// shared catalog) and derives collection counts from the embedded files.
package embeddedillustrations

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested illustration collection or name does not exist.
var ErrNotFound = errors.New("illustration not found")

// providerName is the stable registry key for the embedded illustration provider.
const providerName = "embedded-illustrations"

const (
	dataDir = "data"
	svgExt  = ".svg"
)

// collectionLicense is the license shared by every vendored collection; all are CC0-1.0 with no
// required attribution.
var collectionLicense = assetcore.License{SPDX: "CC0-1.0"}

// illustrationMeta identifies a single illustration within a collection.
type illustrationMeta struct {
	collection string
	name       string
}

var (
	loadOnce    sync.Once
	collections []string
	collSet     map[string]bool
	loadErr     error
)

// load discovers the vendored illustration collections, once, capturing a data directory read
// failure in loadErr so New can distinguish a corrupt embed from a legitimately empty one.
func load() {
	loadOnce.Do(func() {
		entries, err := fs.ReadDir(dataFS, dataDir)
		if err != nil {
			loadErr = fmt.Errorf("read data dir: %w", err)
			collections = []string{}
			collSet = map[string]bool{}
			return
		}
		set := make(map[string]bool, len(entries))
		list := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			list = append(list, e.Name())
			set[e.Name()] = true
		}
		sort.Strings(list)
		collections = list
		collSet = set
	})
}

// loadedCollections returns the sorted list of vendored illustration collection names.
func loadedCollections() []string {
	load()
	return collections
}

// searchIllustrations returns illustrations whose name contains query (case-insensitive) across the
// collections allowed by the sources filter, sorted by (collection, name) and capped at limit.
func searchIllustrations(query string, sources assetcore.Filter, limit int) []illustrationMeta {
	load()

	lowerQuery := strings.ToLower(query)
	var results []illustrationMeta
	for _, col := range collections {
		if !sources.Allows(col) {
			continue
		}
		results = append(results, searchCollection(col, lowerQuery)...)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].collection != results[j].collection {
			return results[i].collection < results[j].collection
		}
		return results[i].name < results[j].name
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// searchCollection returns the illustrations in col whose lowercased stem contains lowerQuery.
func searchCollection(col, lowerQuery string) []illustrationMeta {
	entries, err := fs.ReadDir(dataFS, path.Join(dataDir, col))
	if err != nil {
		return nil
	}

	var results []illustrationMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), svgExt) {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), svgExt)
		if strings.Contains(strings.ToLower(stem), lowerQuery) {
			results = append(results, illustrationMeta{collection: col, name: stem})
		}
	}
	return results
}

// countCollection returns the number of .svg files in col, or -1 if it cannot be read.
func countCollection(col string) int {
	entries, err := fs.ReadDir(dataFS, path.Join(dataDir, col))
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), svgExt) {
			n++
		}
	}
	return n
}

// splitLocal splits a provider-local illustration id ("<collection>/<name>") into its collection and
// name at the first slash. Collection and illustration names never contain a slash.
func splitLocal(id string) (collection, name string, ok bool) {
	collection, name, _ = strings.Cut(id, "/")
	if collection == "" || name == "" {
		return "", "", false
	}
	return collection, name, true
}

// getIllustration returns the raw SVG bytes for the given illustration.
func getIllustration(collection, name string) ([]byte, error) {
	load()

	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid illustration name %q: %w", name, ErrNotFound)
	}
	if !collSet[collection] {
		return nil, fmt.Errorf("unknown illustration collection %q: %w", collection, ErrNotFound)
	}

	data, err := dataFS.ReadFile(path.Join(dataDir, collection, name+svgExt))
	if err != nil {
		return nil, fmt.Errorf("illustration %s/%s: %w", collection, name, ErrNotFound)
	}
	return data, nil
}

// Provider satisfies assetcore.IllustrationProvider and the optional assetcore.SourceLister capability.
var (
	_ assetcore.IllustrationProvider = (*Provider)(nil)
	_ assetcore.SourceLister         = (*Provider)(nil)
)

// Provider serves the vendored SVG illustration collections as an assetcore.IllustrationProvider.
type Provider struct{}

// New returns an embedded illustration provider. It panics if the vendored illustration data fails to
// read, since that indicates a corrupt embed rather than a legitimately empty data directory.
func New() *Provider {
	load()
	if loadErr != nil {
		panic(fmt.Errorf("embeddedillustrations: load data: %w", loadErr))
	}
	return &Provider{}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves illustrations.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIllustration }

// Search finds illustrations matching opts.Query within the collections allowed by opts.Sources and
// maps each hit onto an assetcore.Asset. The embedded catalogue fits in a single page, so NextCursor
// is always "".
func (p *Provider) Search(_ context.Context, opts assetcore.SearchOpts) (assetcore.SearchResult, error) {
	results := searchIllustrations(opts.Query, opts.Sources, assetcore.ClampLimit(opts.Limit))

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.collection, m.name))
	}

	return assetcore.SearchResult{Assets: assets}, nil
}

// Fetch returns the SVG identified by the provider-local id "<collection>/<name>". A malformed id,
// unknown collection, or unknown name is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, id string) (assetcore.Blob, error) {
	collection, name, ok := splitLocal(id)
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("%w: malformed illustration id %q", assetcore.ErrNotFound, id)
	}

	data, err := getIllustration(collection, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	return assetcore.Blob{
		Asset:       p.asset(collection, name),
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}, nil
}

// Sources reports one assetcore.Source per vendored collection, with its license and its file count.
func (p *Provider) Sources() []assetcore.Source {
	cols := loadedCollections()
	out := make([]assetcore.Source, 0, len(cols))
	for _, col := range cols {
		out = append(out, assetcore.Source{
			Name:    col,
			License: collectionLicense,
			Count:   countCollection(col),
		})
	}
	return out
}

// asset builds the assetcore.Asset for an illustration, stamping the composite id and license.
func (p *Provider) asset(collection, name string) assetcore.Asset {
	return assetcore.Asset{
		Source:  collection,
		ID:      assetcore.AssetID(providerName, collection+"/"+name),
		Kind:    assetcore.KindIllustration,
		Title:   name,
		License: collectionLicense,
	}
}

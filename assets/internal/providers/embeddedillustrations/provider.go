// Package embeddedillustrations serves the vendored SVG illustration collections as an
// assetcore.IllustrationProvider.
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
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested illustration collection or name does not exist.
var ErrNotFound = errors.New("illustration not found")

// providerName is the stable registry key for the embedded illustration provider.
const providerName = "embedded-illustrations"

const (
	defaultLimit = 50
	maxLimit     = 200
	dataDir      = "data"
	svgExt       = ".svg"
)

// illustrationMeta identifies a single illustration within a collection.
type illustrationMeta struct {
	collection string
	name       string
}

var (
	loadOnce    sync.Once
	collections []string
	collSet     map[string]bool
)

func load() {
	loadOnce.Do(func() {
		entries, err := fs.ReadDir(dataFS, dataDir)
		if err != nil {
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

// searchIllustrations returns illustrations whose name contains query (case-insensitive),
// optionally filtered to a single collection. limit defaults to 50 and is capped at 200.
func searchIllustrations(query, collection string, limit int) []illustrationMeta {
	load()

	if limit <= 0 {
		limit = defaultLimit
	} else if limit > maxLimit {
		limit = maxLimit
	}

	var cols []string
	if collection != "" {
		if !collSet[collection] {
			return nil
		}
		cols = []string{collection}
	} else {
		cols = collections
	}

	lowerQuery := strings.ToLower(query)
	var results []illustrationMeta
	for _, col := range cols {
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

// Provider satisfies assetcore.IllustrationProvider.
var _ assetcore.IllustrationProvider = (*Provider)(nil)

// Provider serves the vendored SVG illustration collections as an assetcore.IllustrationProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded illustration provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves illustrations.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIllustration }

// Search finds illustrations matching q and maps each hit onto an assetcore.Asset.
func (p *Provider) Search(_ context.Context, q assetcore.IllustrationQuery) (assetcore.Page, error) {
	results := searchIllustrations(q.Query, q.Collection, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.collection, m.name))
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch returns the SVG identified by a (Source=collection, Title=name). An unknown collection or
// name is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	collection, name := a.Source, a.Title

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

// asset builds the assetcore.Asset for an illustration, resolving its license from the catalog.
func (p *Provider) asset(collection, name string) assetcore.Asset {
	license, attribution, _ := p.catalog.IllustrationLicense(collection)

	return assetcore.Asset{
		Provider: providerName,
		Source:   collection,
		ID:       collection + "/" + name,
		Kind:     assetcore.KindIllustration,
		Title:    name,
		License:  assetcore.License{SPDX: license, Attribution: attribution},
	}
}

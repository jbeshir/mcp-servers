// Package illustrations serves vendored SVG illustration collections.
package illustrations

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested illustration collection or name does not exist.
var ErrNotFound = errors.New("illustration not found")

// Meta identifies a single illustration within a collection.
type Meta struct {
	Collection string
	Name       string
}

const (
	defaultLimit = 50
	maxLimit     = 200
	dataDir      = "data"
	svgExt       = ".svg"
)

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

// Collections returns the sorted list of vendored illustration collection names.
func Collections() []string {
	load()
	return collections
}

// Search returns illustrations whose name contains query (case-insensitive), optionally filtered
// to a single collection. limit defaults to 50 and is capped at 200.
func Search(query, collection string, limit int) []Meta {
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
	var results []Meta
	for _, col := range cols {
		results = append(results, searchCollection(col, lowerQuery)...)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Collection != results[j].Collection {
			return results[i].Collection < results[j].Collection
		}
		return results[i].Name < results[j].Name
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// searchCollection returns the illustrations in col whose lowercased stem contains lowerQuery.
func searchCollection(col, lowerQuery string) []Meta {
	entries, err := fs.ReadDir(dataFS, path.Join(dataDir, col))
	if err != nil {
		return nil
	}

	var results []Meta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), svgExt) {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), svgExt)
		if strings.Contains(strings.ToLower(stem), lowerQuery) {
			results = append(results, Meta{Collection: col, Name: stem})
		}
	}
	return results
}

// Get returns the raw SVG bytes for the given illustration.
func Get(collection, name string) ([]byte, error) {
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

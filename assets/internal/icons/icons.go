// Package icons serves vendored Iconify icon sets, rendering individual icons to standalone SVG.
package icons

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed data/*.json
var dataFS embed.FS

// ErrNotFound is returned when a requested icon set or icon name does not exist.
var ErrNotFound = errors.New("icon not found")

// Meta identifies a single icon within a set.
type Meta struct {
	Set  string
	Name string
}

const (
	dataDir            = "data"
	jsonExt            = ".json"
	svgNamespace       = "http://www.w3.org/2000/svg"
	defaultGrid        = 24
	defaultSearchLimit = 50
	maxSearchLimit     = 200
)

// iconData is a single icon entry in the Iconify JSON schema. Only the fields needed for
// rendering are decoded; info, rotate, hFlip, vFlip and other metadata fields are ignored.
type iconData struct {
	Body   string `json:"body"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
	Left   *int   `json:"left"`
	Top    *int   `json:"top"`
}

// aliasData is an alias entry that points at a parent icon, optionally overriding its grid.
type aliasData struct {
	Parent string `json:"parent"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
	Left   *int   `json:"left"`
	Top    *int   `json:"top"`
}

// iconSet is the top-level shape of a vendored data/<set>.json file.
type iconSet struct {
	Prefix  string               `json:"prefix"`
	Width   int                  `json:"width"`
	Height  int                  `json:"height"`
	Icons   map[string]iconData  `json:"icons"`
	Aliases map[string]aliasData `json:"aliases"`
}

var (
	loadOnce sync.Once
	sets     map[string]iconSet
)

// load parses every embedded data/<set>.json file into sets, once.
func load() {
	loadOnce.Do(func() {
		sets = make(map[string]iconSet)
		entries, err := dataFS.ReadDir(dataDir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonExt) {
				continue
			}
			raw, err := dataFS.ReadFile(path.Join(dataDir, entry.Name()))
			if err != nil {
				continue
			}
			var s iconSet
			if err := json.Unmarshal(raw, &s); err != nil {
				continue
			}
			sets[strings.TrimSuffix(entry.Name(), jsonExt)] = s
		}
	})
}

// Sets returns the sorted list of vendored icon set names.
func Sets() []string {
	load()
	names := make([]string, 0, len(sets))
	for name := range sets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Search returns icons whose name contains query (case-insensitive), optionally filtered to a
// single set. limit defaults to 50 and is capped at 200.
func Search(query, set string, limit int) []Meta {
	load()
	limit = clampLimit(limit)
	setNames := Sets()
	if set != "" {
		if _, ok := sets[set]; !ok {
			return nil
		}
		setNames = []string{set}
	}
	q := strings.ToLower(query)
	var results []Meta
	for _, name := range setNames {
		for iconName := range sets[name].Icons {
			if strings.Contains(strings.ToLower(iconName), q) {
				results = append(results, Meta{Set: name, Name: iconName})
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Set != results[j].Set {
			return results[i].Set < results[j].Set
		}
		return results[i].Name < results[j].Name
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSearchLimit
	case limit > maxSearchLimit:
		return maxSearchLimit
	default:
		return limit
	}
}

// Render returns the standalone SVG bytes for the given icon, with an optional color override and
// output size. Rotation and horizontal/vertical flip transforms present on some alias entries are
// not applied; only body, dimensions and offset are resolved.
func Render(set, name, color string, size int) ([]byte, error) {
	load()
	s, ok := sets[set]
	if !ok {
		return nil, fmt.Errorf("icon set %q: %w", set, ErrNotFound)
	}
	body, width, height, left, top, ok := resolveIcon(s, name)
	if !ok {
		return nil, fmt.Errorf("icon %q in set %q: %w", name, set, ErrNotFound)
	}
	outWidth, outHeight := width, height
	if size > 0 {
		outWidth, outHeight = size, size
	}
	var colorAttr string
	if color != "" {
		colorAttr = fmt.Sprintf(" color=%q", color)
	}
	svg := fmt.Sprintf(
		`<svg xmlns=%q width="%d" height="%d" viewBox="%d %d %d %d"%s>%s</svg>`,
		svgNamespace, outWidth, outHeight, left, top, width, height, colorAttr, body,
	)
	return []byte(svg), nil
}

// resolveIcon looks up name directly, then as an alias of a parent icon, returning its body and
// resolved grid (width, height, left, top). ok is false if name is not found in either map.
func resolveIcon(s iconSet, name string) (body string, width, height, left, top int, ok bool) {
	grid, gridH := setGrid(s.Width), setGrid(s.Height)
	if icon, exists := s.Icons[name]; exists {
		return icon.Body, firstInt(grid, icon.Width), firstInt(gridH, icon.Height),
			firstInt(0, icon.Left), firstInt(0, icon.Top), true
	}
	alias, exists := s.Aliases[name]
	if !exists {
		return "", 0, 0, 0, 0, false
	}
	parent, exists := s.Icons[alias.Parent]
	if !exists {
		return "", 0, 0, 0, 0, false
	}
	width = firstInt(grid, alias.Width, parent.Width)
	height = firstInt(gridH, alias.Height, parent.Height)
	left = firstInt(0, alias.Left, parent.Left)
	top = firstInt(0, alias.Top, parent.Top)
	return parent.Body, width, height, left, top, true
}

// setGrid returns v, or the Iconify default 24x24 grid if v is unset (zero).
func setGrid(v int) int {
	if v != 0 {
		return v
	}
	return defaultGrid
}

// firstInt returns the first non-nil value among vals, or fallback if all are nil.
func firstInt(fallback int, vals ...*int) int {
	for _, v := range vals {
		if v != nil {
			return *v
		}
	}
	return fallback
}

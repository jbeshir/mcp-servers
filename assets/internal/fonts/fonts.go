// Package fonts serves vendored OFL-1.1 font families as woff2 files with optional @font-face CSS.
package fonts

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested font family, weight, or style does not exist.
var ErrNotFound = errors.New("font not found")

// Meta describes a font family and the weights available for it.
type Meta struct {
	Family   string
	Slug     string
	Category string
	Weights  []int
}

const (
	dataDir       = "data"
	latinInfix    = "-latin-"
	normalSuffix  = "-normal.woff2"
	defaultWeight = 400
	styleNormal   = "normal"
	defaultLimit  = 50
	maxLimit      = 200

	categorySans    = "sans"
	categorySerif   = "serif"
	categoryMono    = "mono"
	categoryDisplay = "display"
)

// familyInfo is the human-readable metadata for a font family, keyed by slug.
type familyInfo struct {
	family   string
	category string
}

// familyLookup maps each vendored font slug to its display name and category. Slugifying the
// display name (lowercase, spaces to hyphens) always yields the slug, so a single lookup by
// slugified input resolves both display names and slugs.
var familyLookup = map[string]familyInfo{
	"inter":            {"Inter", categorySans},
	"roboto":           {"Roboto", categorySans},
	"open-sans":        {"Open Sans", categorySans},
	"lato":             {"Lato", categorySans},
	"montserrat":       {"Montserrat", categorySans},
	"poppins":          {"Poppins", categorySans},
	"source-serif-4":   {"Source Serif 4", categorySerif},
	"merriweather":     {"Merriweather", categorySerif},
	"lora":             {"Lora", categorySerif},
	"playfair-display": {"Playfair Display", categorySerif},
	"jetbrains-mono":   {"JetBrains Mono", categoryMono},
	"fira-code":        {"Fira Code", categoryMono},
	"ibm-plex-mono":    {"IBM Plex Mono", categoryMono},
	"bebas-neue":       {"Bebas Neue", categoryDisplay},
}

var (
	loadOnce   sync.Once
	families   []Meta
	familyData map[string]Meta
)

// load discovers the vendored font families by reading the embedded data/ subdirectories, once.
func load() {
	loadOnce.Do(func() {
		familyData = make(map[string]Meta, len(familyLookup))

		entries, err := fs.ReadDir(dataFS, dataDir)
		if err != nil {
			families = []Meta{}
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			slug := e.Name()

			info, ok := familyLookup[slug]
			if !ok {
				continue
			}

			weights := weightsForSlug(slug)
			if len(weights) == 0 {
				continue
			}

			familyData[slug] = Meta{Family: info.family, Slug: slug, Category: info.category, Weights: weights}
		}

		families = make([]Meta, 0, len(familyData))
		for _, m := range familyData {
			families = append(families, m)
		}

		sort.Slice(families, func(i, j int) bool { return families[i].Family < families[j].Family })
	})
}

// weightsForSlug parses data/<slug>/<slug>-latin-<weight>-normal.woff2 filenames to discover the
// weights vendored for a font family, sorted ascending.
func weightsForSlug(slug string) []int {
	entries, err := fs.ReadDir(dataFS, path.Join(dataDir, slug))
	if err != nil {
		return nil
	}

	prefix := slug + latinInfix

	var weights []int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, normalSuffix) {
			continue
		}

		weightStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), normalSuffix)

		w, err := strconv.Atoi(weightStr)
		if err != nil {
			continue
		}

		weights = append(weights, w)
	}

	sort.Ints(weights)

	return weights
}

// slugify normalizes a font family display name or slug into its canonical slug form.
func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

// Families returns the vendored font families, sorted by Family.
func Families() []Meta {
	load()

	return families
}

// Search returns font families whose Family, Slug, or Category contains query (case-insensitive).
// limit defaults to 50 and is capped at 200.
func Search(query string, limit int) []Meta {
	load()

	if limit <= 0 {
		limit = defaultLimit
	} else if limit > maxLimit {
		limit = maxLimit
	}

	q := strings.ToLower(query)

	var results []Meta
	for _, m := range families {
		if strings.Contains(strings.ToLower(m.Family), q) ||
			strings.Contains(strings.ToLower(m.Slug), q) ||
			strings.Contains(strings.ToLower(m.Category), q) {
			results = append(results, m)
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// Font is a single rendered font file variant.
type Font struct {
	Filename string
	Data     []byte
	Weight   int
	Style    string
}

// Get returns the woff2 bytes for the given family, weight, and style. weight defaults to 400 and
// style defaults to "normal".
func Get(family string, weight int, style string) (Font, error) {
	load()

	if weight == 0 {
		weight = defaultWeight
	}

	if style == "" {
		style = styleNormal
	}

	if style != styleNormal {
		return Font{}, fmt.Errorf("font style %q: %w", style, ErrNotFound)
	}

	slug := slugify(family)

	meta, ok := familyData[slug]
	if !ok {
		return Font{}, fmt.Errorf("font family %q: %w", family, ErrNotFound)
	}

	if !hasWeight(meta.Weights, weight) {
		return Font{}, fmt.Errorf("font family %q has no weight %d: %w", family, weight, ErrNotFound)
	}

	filename := fmt.Sprintf("%s-latin-%d-%s.woff2", slug, weight, style)

	data, err := dataFS.ReadFile(path.Join(dataDir, slug, filename))
	if err != nil {
		return Font{}, fmt.Errorf("read font file %s: %w", filename, err)
	}

	return Font{Filename: filename, Data: data, Weight: weight, Style: style}, nil
}

func hasWeight(weights []int, weight int) bool {
	for _, w := range weights {
		if w == weight {
			return true
		}
	}

	return false
}

// FontFace returns an @font-face CSS snippet referencing f's filename, for use under familyDisplay.
func FontFace(familyDisplay string, f Font) string {
	return fmt.Sprintf(
		"@font-face {\n  font-family: %q;\n  font-style: %s;\n  font-weight: %d;\n  src: url(%q) format(\"woff2\");\n}\n",
		familyDisplay, f.Style, f.Weight, f.Filename,
	)
}

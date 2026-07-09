// Package embeddedfonts serves the vendored OFL-1.1 font families as woff2 files with optional
// @font-face CSS, implementing assetcore.FontProvider and assetcore.FontFaceRenderer.
package embeddedfonts

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested font family, weight, or style does not exist.
var ErrNotFound = errors.New("font not found")

// providerName is the stable registry key for the embedded font provider.
const providerName = "embedded-fonts"

// woff2ContentType is the MIME type of the vendored font files.
const woff2ContentType = "font/woff2"

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

// fontFamily describes a font family and the weights available for it.
type fontFamily struct {
	family   string
	slug     string
	category string
	weights  []int
}

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
	families   []fontFamily
	familyData map[string]fontFamily
)

// load discovers the vendored font families by reading the embedded data/ subdirectories, once.
func load() {
	loadOnce.Do(func() {
		familyData = make(map[string]fontFamily, len(familyLookup))

		entries, err := fs.ReadDir(dataFS, dataDir)
		if err != nil {
			families = []fontFamily{}
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

			familyData[slug] = fontFamily{family: info.family, slug: slug, category: info.category, weights: weights}
		}

		families = make([]fontFamily, 0, len(familyData))
		for _, m := range familyData {
			families = append(families, m)
		}

		sort.Slice(families, func(i, j int) bool { return families[i].family < families[j].family })
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

// loadedFamilies returns the vendored font families, sorted by family.
func loadedFamilies() []fontFamily {
	load()

	return families
}

// searchFonts returns font families whose family, slug, or category contains query
// (case-insensitive). limit defaults to 50 and is capped at 200.
func searchFonts(query string, limit int) []fontFamily {
	load()

	if limit <= 0 {
		limit = defaultLimit
	} else if limit > maxLimit {
		limit = maxLimit
	}

	q := strings.ToLower(query)

	var results []fontFamily
	for _, m := range families {
		if strings.Contains(strings.ToLower(m.family), q) ||
			strings.Contains(strings.ToLower(m.slug), q) ||
			strings.Contains(strings.ToLower(m.category), q) {
			results = append(results, m)
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// fontFile is a single rendered font file variant.
type fontFile struct {
	filename string
	data     []byte
	weight   int
	style    string
}

// getFont returns the woff2 bytes for the given family, weight, and style. weight defaults to 400
// and style defaults to "normal".
func getFont(family string, weight int, style string) (fontFile, error) {
	load()

	if weight == 0 {
		weight = defaultWeight
	}

	if style == "" {
		style = styleNormal
	}

	if style != styleNormal {
		return fontFile{}, fmt.Errorf("font style %q: %w", style, ErrNotFound)
	}

	slug := slugify(family)

	meta, ok := familyData[slug]
	if !ok {
		return fontFile{}, fmt.Errorf("font family %q: %w", family, ErrNotFound)
	}

	if !hasWeight(meta.weights, weight) {
		return fontFile{}, fmt.Errorf("font family %q has no weight %d: %w", family, weight, ErrNotFound)
	}

	filename := fmt.Sprintf("%s-latin-%d-%s.woff2", slug, weight, style)

	data, err := dataFS.ReadFile(path.Join(dataDir, slug, filename))
	if err != nil {
		return fontFile{}, fmt.Errorf("read font file %s: %w", filename, err)
	}

	return fontFile{filename: filename, data: data, weight: weight, style: style}, nil
}

func hasWeight(weights []int, weight int) bool {
	for _, w := range weights {
		if w == weight {
			return true
		}
	}

	return false
}

// fontFaceCSS returns an @font-face CSS snippet referencing f's filename, for use under
// familyDisplay.
func fontFaceCSS(familyDisplay string, f fontFile) string {
	return fmt.Sprintf(
		"@font-face {\n  font-family: %q;\n  font-style: %s;\n  font-weight: %d;\n  src: url(%q) format(\"woff2\");\n}\n",
		familyDisplay, f.style, f.weight, f.filename,
	)
}

// Provider satisfies assetcore.FontProvider and the optional assetcore.FontFaceRenderer capability.
var (
	_ assetcore.FontProvider     = (*Provider)(nil)
	_ assetcore.FontFaceRenderer = (*Provider)(nil)
)

// Provider serves the vendored OFL font families as an assetcore.FontProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded font provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves fonts.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindFont }

// Search finds font families matching q and maps each hit onto an assetcore.Asset, carrying the
// family's category and available weights as Ref hints for search-result display.
func (p *Provider) Search(_ context.Context, q assetcore.FontQuery) (assetcore.Page, error) {
	results := searchFonts(q.Query, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		license, attribution, _ := p.catalog.FontLicense(m.family)

		weights := make([]string, 0, len(m.weights))
		for _, w := range m.weights {
			weights = append(weights, strconv.Itoa(w))
		}

		assets = append(assets, assetcore.Asset{
			Provider: providerName,
			Source:   m.slug,
			ID:       m.slug,
			Kind:     assetcore.KindFont,
			Title:    m.family,
			License:  assetcore.License{SPDX: license, Attribution: attribution},
			Ref: map[string]string{
				assetcore.RefCategory: m.category,
				assetcore.RefWeights:  strings.Join(weights, ","),
			},
		})
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch returns the woff2 identified by a (Source=family) with the weight/style hints carried in
// a.Ref. The Blob's Filename is the internal font filename (referenced by @font-face CSS), not the
// caller's output filename. An unknown family/weight/style is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	family := a.Source
	weight, _ := strconv.Atoi(a.Ref[assetcore.RefWeight])
	style := a.Ref[assetcore.RefStyle]

	f, err := getFont(family, weight, style)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	license, attribution, _ := p.catalog.FontLicense(family)

	asset := a
	asset.Provider = providerName
	asset.Kind = assetcore.KindFont
	asset.License = assetcore.License{SPDX: license, Attribution: attribution}

	return assetcore.Blob{
		Asset:       asset,
		Content:     f.data,
		Filename:    f.filename,
		ContentType: woff2ContentType,
	}, nil
}

// RenderFontFace renders an @font-face CSS snippet for a font Blob produced by Fetch, reconstructing
// the fontFile from the Blob's internal filename and the weight/style hints on the Blob's Asset. It
// satisfies assetcore.FontFaceRenderer.
func (p *Provider) RenderFontFace(familyDisplay string, b assetcore.Blob) string {
	weight, _ := strconv.Atoi(b.Asset.Ref[assetcore.RefWeight])
	style := b.Asset.Ref[assetcore.RefStyle]

	return fontFaceCSS(familyDisplay, fontFile{
		filename: b.Filename,
		weight:   weight,
		style:    style,
	})
}

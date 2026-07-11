// Package embeddedfonts serves the vendored OFL-1.1 font families as woff2 files with optional
// @font-face CSS, implementing assetcore.FontProvider, assetcore.FontFaceRenderer, and
// assetcore.SourceLister. It owns its per-family license and category metadata (folded from the former
// shared catalog) and derives variant counts from the embedded files.
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
)

//go:embed data
var dataFS embed.FS

// ErrNotFound is returned when a requested font family, weight, or style does not exist.
var ErrNotFound = errors.New("font not found")

// providerName is the stable registry key for the embedded font provider.
const providerName = "embedded-fonts"

// woff2ContentType is the MIME type of the vendored font files.
const woff2ContentType = "font/woff2"

// fontLicense is the license shared by every vendored family; all are distributed under OFL-1.1 with
// no required attribution (each family's LICENSE file travels in the repo, not the served payload).
var fontLicense = assetcore.License{SPDX: "OFL-1.1"}

const (
	dataDir       = "data"
	latinInfix    = "-latin-"
	normalSuffix  = "-normal.woff2"
	woff2Suffix   = ".woff2"
	defaultWeight = 400
	styleNormal   = "normal"

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
// (case-insensitive) among the families allowed by the sources filter, capped at limit.
func searchFonts(query string, sources assetcore.Filter, limit int) []fontFamily {
	load()

	q := strings.ToLower(query)

	var results []fontFamily
	for _, m := range families {
		if !sources.Allows(m.family) {
			continue
		}
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

// parseFontFilename recovers the weight and style encoded in a vendored font filename
// (<slug>-latin-<weight>-<style>.woff2), so @font-face CSS can be rendered from a Blob alone.
func parseFontFilename(filename string) (weight int, style string) {
	base := strings.TrimSuffix(filename, woff2Suffix)
	idx := strings.LastIndex(base, latinInfix)
	if idx < 0 {
		return 0, ""
	}

	weightStr, style, ok := strings.Cut(base[idx+len(latinInfix):], "-")
	if !ok {
		return 0, ""
	}

	w, err := strconv.Atoi(weightStr)
	if err != nil {
		return 0, ""
	}

	return w, style
}

// fontFaceCSS returns an @font-face CSS snippet referencing f's filename, for use under
// familyDisplay.
func fontFaceCSS(familyDisplay string, f fontFile) string {
	return fmt.Sprintf(
		"@font-face {\n  font-family: %q;\n  font-style: %s;\n  font-weight: %d;\n  src: url(%q) format(\"woff2\");\n}\n",
		familyDisplay, f.style, f.weight, f.filename,
	)
}

// Provider satisfies assetcore.FontProvider and the optional FontFaceRenderer/SourceLister capabilities.
var (
	_ assetcore.FontProvider     = (*Provider)(nil)
	_ assetcore.FontFaceRenderer = (*Provider)(nil)
	_ assetcore.SourceLister     = (*Provider)(nil)
)

// Provider serves the vendored OFL font families as an assetcore.FontProvider.
type Provider struct{}

// New returns an embedded font provider.
func New() *Provider { return &Provider{} }

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves fonts.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindFont }

// Search finds font families matching opts.Query among those allowed by opts.Sources and maps each
// hit onto an assetcore.Asset, carrying the family's category and available weights as display Meta.
func (p *Provider) Search(_ context.Context, opts assetcore.SearchOpts) ([]assetcore.Asset, error) {
	results := searchFonts(opts.Query, opts.Sources, assetcore.ClampLimit(opts.Limit))

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m))
	}

	return assets, nil
}

// Fetch returns the woff2 for the family identified by the provider-local slug id, at the weight and
// style in opts (defaulting to 400/normal). The Blob's Filename is the internal font filename
// (referenced by @font-face CSS), not the caller's output filename. An unknown family/weight/style is
// reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, id string, opts assetcore.FontFetchOpts) (assetcore.Blob, error) {
	f, err := getFont(id, opts.Weight, opts.Style)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	meta := familyData[slugify(id)]

	return assetcore.Blob{
		Asset: assetcore.Asset{
			Source:  meta.family,
			ID:      assetcore.AssetID(providerName, meta.slug),
			Kind:    assetcore.KindFont,
			Title:   meta.family,
			License: fontLicense,
		},
		Content:     f.data,
		Filename:    f.filename,
		ContentType: woff2ContentType,
	}, nil
}

// RenderFontFace renders an @font-face CSS snippet for a font Blob produced by Fetch, recovering the
// weight and style from the Blob's internal filename. It satisfies assetcore.FontFaceRenderer.
func (p *Provider) RenderFontFace(familyDisplay string, b assetcore.Blob) string {
	weight, style := parseFontFilename(b.Filename)

	return fontFaceCSS(familyDisplay, fontFile{
		filename: b.Filename,
		weight:   weight,
		style:    style,
	})
}

// Sources reports one assetcore.Source per vendored family, with its license, its variant count, and
// its category as display Meta.
func (p *Provider) Sources() []assetcore.Source {
	fams := loadedFamilies()
	out := make([]assetcore.Source, 0, len(fams))
	for _, m := range fams {
		out = append(out, assetcore.Source{
			Name:    m.family,
			License: fontLicense,
			Count:   len(m.weights),
			Meta:    map[string]string{assetcore.MetaCategory: m.category},
		})
	}
	return out
}

// asset builds the search-hit assetcore.Asset for a family, carrying category and weights as Meta.
func (p *Provider) asset(m fontFamily) assetcore.Asset {
	weights := make([]string, 0, len(m.weights))
	for _, w := range m.weights {
		weights = append(weights, strconv.Itoa(w))
	}

	return assetcore.Asset{
		Source:  m.family,
		ID:      assetcore.AssetID(providerName, m.slug),
		Kind:    assetcore.KindFont,
		Title:   m.family,
		License: fontLicense,
		Meta: map[string]string{
			assetcore.MetaCategory: m.category,
			assetcore.MetaWeights:  strings.Join(weights, ","),
		},
	}
}

// Package embeddedicons serves the vendored Iconify icon sets as an assetcore.IconProvider,
// rendering individual icons to standalone SVG.
package embeddedicons

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
)

//go:embed data/*.json
var dataFS embed.FS

// ErrNotFound is returned when a requested icon set or icon name does not exist.
var ErrNotFound = errors.New("icon not found")

// providerName is the stable registry key for the embedded icon provider.
const providerName = "embedded-icons"

const (
	dataDir            = "data"
	jsonExt            = ".json"
	svgNamespace       = "http://www.w3.org/2000/svg"
	defaultGrid        = 24
	defaultSearchLimit = 50
	maxSearchLimit     = 200
)

// iconMeta identifies a single icon within a set.
type iconMeta struct {
	set  string
	name string
}

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
	iconSets map[string]iconSet
)

// load parses every embedded data/<set>.json file into iconSets, once.
func load() {
	loadOnce.Do(func() {
		iconSets = make(map[string]iconSet)
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
			iconSets[strings.TrimSuffix(entry.Name(), jsonExt)] = s
		}
	})
}

// loadedSetNames returns the sorted list of vendored icon set names.
func loadedSetNames() []string {
	load()
	names := make([]string, 0, len(iconSets))
	for name := range iconSets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// searchIcons returns icons whose name contains query (case-insensitive), optionally filtered to a
// single set. limit defaults to 50 and is capped at 200.
func searchIcons(query, set string, limit int) []iconMeta {
	load()
	limit = clampLimit(limit)
	setNames := loadedSetNames()
	if set != "" {
		if _, ok := iconSets[set]; !ok {
			return nil
		}
		setNames = []string{set}
	}
	q := strings.ToLower(query)
	var results []iconMeta
	for _, name := range setNames {
		for iconName := range iconSets[name].Icons {
			if strings.Contains(strings.ToLower(iconName), q) {
				results = append(results, iconMeta{set: name, name: iconName})
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].set != results[j].set {
			return results[i].set < results[j].set
		}
		return results[i].name < results[j].name
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

// renderIcon returns the standalone SVG bytes for the given icon, with an optional color override
// and output size. Rotation and horizontal/vertical flip transforms present on some alias entries
// are not applied; only body, dimensions and offset are resolved.
func renderIcon(set, name, color string, size int) ([]byte, error) {
	load()
	s, ok := iconSets[set]
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

// Provider satisfies assetcore.IconProvider.
var _ assetcore.IconProvider = (*Provider)(nil)

// Provider serves the vendored Iconify icon sets as an assetcore.IconProvider.
type Provider struct {
	catalog *catalog.Catalog
}

// New returns an embedded icon provider that resolves licensing through cat.
func New(cat *catalog.Catalog) *Provider {
	return &Provider{catalog: cat}
}

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves icons.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIcon }

// Search finds icons matching q and maps each hit onto an assetcore.Asset.
func (p *Provider) Search(_ context.Context, q assetcore.IconQuery) (assetcore.Page, error) {
	results := searchIcons(q.Query, q.Set, q.Limit)

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.set, m.name))
	}

	return assetcore.Page{Assets: assets, Total: len(assets)}, nil
}

// Fetch renders the icon identified by a (Source=set, Title=name) with the colour/size hints
// carried in a.Ref. An unknown set or icon is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, a assetcore.Asset) (assetcore.Blob, error) {
	set, name := a.Source, a.Title
	color := a.Ref[assetcore.RefColor]
	size, _ := strconv.Atoi(a.Ref[assetcore.RefSize])

	data, err := renderIcon(set, name, color, size)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	asset := p.asset(set, name)
	asset.Ref = a.Ref

	return assetcore.Blob{
		Asset:       asset,
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}, nil
}

// asset builds the assetcore.Asset for an icon, resolving its license from the catalog.
func (p *Provider) asset(set, name string) assetcore.Asset {
	license, attribution, _ := p.catalog.IconLicense(set)

	return assetcore.Asset{
		Provider: providerName,
		Source:   set,
		ID:       set + "/" + name,
		Kind:     assetcore.KindIcon,
		Title:    name,
		License:  assetcore.License{SPDX: license, Attribution: attribution},
	}
}

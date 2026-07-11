// Package embeddedicons serves the vendored Iconify icon sets as an assetcore.IconProvider,
// rendering individual icons to standalone SVG. It owns its per-set license metadata (folded from the
// former shared catalog) and derives icon counts from the embedded data, so nothing can drift.
package embeddedicons

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

//go:embed data/*.json
var dataFS embed.FS

// ErrNotFound is returned when a requested icon set or icon name does not exist.
var ErrNotFound = errors.New("icon not found")

// providerName is the stable registry key for the embedded icon provider.
const providerName = "embedded-icons"

const (
	dataDir      = "data"
	jsonExt      = ".json"
	svgNamespace = "http://www.w3.org/2000/svg"
	defaultGrid  = 24
)

// setLicenses is the per-set licensing owned by this provider, folded from the former catalog.json.
// Every vendored set's SPDX identifier lives here; attribution is empty for all of them.
var setLicenses = map[string]assetcore.License{
	"lucide":           {SPDX: "ISC"},
	"heroicons":        {SPDX: "MIT"},
	"tabler":           {SPDX: "MIT"},
	"phosphor":         {SPDX: "MIT"},
	"feather":          {SPDX: "MIT"},
	"bootstrap-icons":  {SPDX: "MIT"},
	"material-symbols": {SPDX: "Apache-2.0"},
	"simple-icons":     {SPDX: "CC0-1.0"},
}

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

// searchIcons returns icons whose name contains query (case-insensitive) across the sets allowed by
// the sources filter, sorted by (set, name) and capped at limit.
func searchIcons(query string, sources assetcore.Filter, limit int) []iconMeta {
	load()
	q := strings.ToLower(query)
	var results []iconMeta
	for _, name := range loadedSetNames() {
		if !sources.Allows(name) {
			continue
		}
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

// splitLocal splits a provider-local icon id ("<set>/<name>") into its set and name at the first
// slash. Icon set and icon names never contain a slash, so a single split is exact.
func splitLocal(id string) (set, name string, ok bool) {
	set, name, _ = strings.Cut(id, "/")
	if set == "" || name == "" {
		return "", "", false
	}
	return set, name, true
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
		colorAttr = " color=\"" + escapeXMLAttr(color) + "\""
	}
	svg := fmt.Sprintf(
		`<svg xmlns=%q width="%d" height="%d" viewBox="%d %d %d %d"%s>%s</svg>`,
		svgNamespace, outWidth, outHeight, left, top, width, height, colorAttr, body,
	)
	return []byte(svg), nil
}

// xmlAttrEscaper escapes the five XML metacharacters so a user-supplied value cannot break out of a
// double-quoted attribute in the rendered SVG.
var xmlAttrEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

// escapeXMLAttr renders s safe for interpolation inside a double-quoted XML attribute value.
func escapeXMLAttr(s string) string { return xmlAttrEscaper.Replace(s) }

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

// Provider satisfies assetcore.IconProvider and the optional assetcore.SourceLister capability.
var (
	_ assetcore.IconProvider = (*Provider)(nil)
	_ assetcore.SourceLister = (*Provider)(nil)
)

// Provider serves the vendored Iconify icon sets as an assetcore.IconProvider.
type Provider struct{}

// New returns an embedded icon provider.
func New() *Provider { return &Provider{} }

// Name returns the provider's stable registry key.
func (p *Provider) Name() string { return providerName }

// Kind reports that this provider serves icons.
func (p *Provider) Kind() assetcore.Kind { return assetcore.KindIcon }

// Search finds icons matching opts.Query within the sets allowed by opts.Sources and maps each hit
// onto an assetcore.Asset.
func (p *Provider) Search(_ context.Context, opts assetcore.SearchOpts) ([]assetcore.Asset, error) {
	results := searchIcons(opts.Query, opts.Sources, assetcore.ClampLimit(opts.Limit))

	assets := make([]assetcore.Asset, 0, len(results))
	for _, m := range results {
		assets = append(assets, p.asset(m.set, m.name))
	}

	return assets, nil
}

// Fetch renders the icon identified by the provider-local id "<set>/<name>", honouring the colour and
// size in opts. A malformed id, unknown set, or unknown icon is reported as assetcore.ErrNotFound.
func (p *Provider) Fetch(_ context.Context, id string, opts assetcore.IconFetchOpts) (assetcore.Blob, error) {
	set, name, ok := splitLocal(id)
	if !ok {
		return assetcore.Blob{}, fmt.Errorf("%w: malformed icon id %q", assetcore.ErrNotFound, id)
	}

	data, err := renderIcon(set, name, opts.Color, opts.Size)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return assetcore.Blob{}, errors.Join(assetcore.ErrNotFound, err)
		}

		return assetcore.Blob{}, err
	}

	return assetcore.Blob{
		Asset:       p.asset(set, name),
		Content:     data,
		Filename:    name + ".svg",
		ContentType: "image/svg+xml",
	}, nil
}

// Sources reports one assetcore.Source per vendored set, with its license and its embedded icon count.
func (p *Provider) Sources() []assetcore.Source {
	names := loadedSetNames()
	out := make([]assetcore.Source, 0, len(names))
	for _, set := range names {
		out = append(out, assetcore.Source{
			Name:    set,
			License: setLicenses[set],
			Count:   len(iconSets[set].Icons),
		})
	}
	return out
}

// asset builds the assetcore.Asset for an icon, stamping the composite id and the set's license.
func (p *Provider) asset(set, name string) assetcore.Asset {
	return assetcore.Asset{
		Source:  set,
		ID:      assetcore.AssetID(providerName, set+"/"+name),
		Kind:    assetcore.KindIcon,
		Title:   name,
		License: setLicenses[set],
	}
}

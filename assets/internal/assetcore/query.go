package assetcore

import "slices"

// Search limit bounds shared by every provider and the search handlers. ClampLimit is the single
// authoritative clamp; providers call it rather than duplicating these constants.
const (
	defaultSearchLimit = 50
	maxSearchLimit     = 200
)

// Filter is a reusable allow/deny list over names (source names, provider names). An empty Only means
// "allow all"; Except always subtracts. Value type; copy freely.
type Filter struct {
	Only   []string // if non-empty, only these names are allowed
	Except []string // these names are always denied
}

// Allows reports whether name passes the filter.
func (f Filter) Allows(name string) bool {
	return (len(f.Only) == 0 || slices.Contains(f.Only, name)) && !slices.Contains(f.Except, name)
}

// SearchOpts holds the search parameters shared by every kind. Weight/style and colour/size are
// fetch-time parameters, not search filters, so they live in the typed Fetch opts, not here. Cursor is
// opaque and provider-scoped.
type SearchOpts struct {
	Query     string   // case-insensitive substring to match
	Licenses  []string // SPDX allow-list hint; providers filter/annotate (unused by embedded providers)
	Cursor    string   // opaque; "" = first page
	Limit     int      // result cap hint; 0 = unset. Providers clamp via ClampLimit.
	Sources   Filter   // scope which upstream sources (sets/collections/families) are returned
	Providers Filter   // scope which providers are searched
}

// ClampLimit applies the default (50) and maximum (200) bounds shared by every search, treating a
// non-positive limit as unset.
func ClampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSearchLimit
	case limit > maxSearchLimit:
		return maxSearchLimit
	default:
		return limit
	}
}

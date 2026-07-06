package assetcore

// SearchOpts holds the search parameters shared by every kind. It is embedded in each typed query so
// per-kind filters can be added without a god-struct. Cursor is opaque and provider-scoped.
type SearchOpts struct {
	Query    string   // case-insensitive substring to match
	Licenses []string // SPDX allow-list hint; providers filter/annotate (unused by embedded providers)
	Cursor   string   // opaque; "" = first page
	Limit    int      // result cap hint; providers clamp to their own bounds
}

// IconQuery is the typed query for icon providers. Set restricts the search to a single icon set,
// mirroring today's search_icons "set" argument.
type IconQuery struct {
	SearchOpts
	Set string // restrict to a single icon set; "" searches all
}

// IllustrationQuery is the typed query for illustration providers. Collection restricts the search to
// a single collection, mirroring today's search_illustrations "collection" argument.
type IllustrationQuery struct {
	SearchOpts
	Collection string // restrict to a single collection; "" searches all
}

// FontQuery is the typed query for font providers. Font weight and style are fetch-time parameters,
// not search filters, so the query stays minimal and faithful to today's search_fonts behaviour,
// which matches the query against family name, slug, and category only.
type FontQuery struct {
	SearchOpts
}

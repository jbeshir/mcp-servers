package assetcore

import "errors"

// ErrNotFound is returned by a provider's Fetch (or Search of an unknown source) when the requested
// asset does not exist. It is the provider-neutral counterpart of the kind packages' own ErrNotFound
// values, so callers depend only on assetcore.
var ErrNotFound = errors.New("asset not found")

// License carries the licensing and attribution of an asset. It is a pre-formatted, ready-to-embed
// record (à la Openverse): the Attribution string is copied verbatim into the output manifest, never
// assembled at the call site. Value type; copy freely.
type License struct {
	SPDX                string // e.g. "CC0-1.0", "OFL-1.1", "MIT"; empty if unknown
	Name                string // human-readable label
	URL                 string // deed / license text URL
	Attribution         string // ready-to-embed attribution; empty if none required
	RequiresAttribution bool
}

// Asset is a search hit: metadata only, no bytes. Fetch takes an Asset back so a provider can reuse
// the identity (and any Ref hints) it emitted during Search. Value type; copy freely.
type Asset struct {
	Provider   string            // registry key of the emitting provider, e.g. "embedded-icons"
	Source     string            // upstream set/collection/family, e.g. "lucide", "open-doodles"
	ID         string            // provider-scoped, Fetch-round-trippable identifier
	Kind       Kind              //
	Title      string            // display name within the source (icon/illustration name, font family)
	Tags       []string          //
	License    License           //
	LandingURL string            // upstream page, for credit/debugging
	PreviewURL string            // thumbnail (display only; not the fetch target)
	Ref        map[string]string // opaque provider hints threaded from Search/caller into Fetch
}

// Well-known Asset.Ref keys. Ref is a stringly-typed bag of provider hints (the brief's chosen
// mechanism for threading non-identity parameters through Fetch, and per-kind display data through
// Search) rather than widening Asset with per-kind fields. These constants are the shared contract
// between the handlers that populate Ref and the embedded providers that read it.
const (
	RefColor    = "color"    // icon render colour override (icon Fetch)
	RefSize     = "size"     // icon render size in px, decimal string, "0"/"" = native grid (icon Fetch)
	RefWeight   = "weight"   // font weight, decimal string (font Fetch)
	RefStyle    = "style"    // font style (font Fetch)
	RefCategory = "category" // font family category, for search display (font Search)
	RefWeights  = "weights"  // comma-separated font weights, for search display (font Search)
)

// Page is one search response. NextCursor is opaque: only the emitting provider parses it.
type Page struct {
	Assets     []Asset
	NextCursor string // "" => no more results
	Total      int    // best-effort; -1 if unknown
}

// Blob is a materialized asset: bytes plus the metadata the writer layer turns into a file and a
// manifest entry. []byte is sufficient for the small icon/illustration/font payloads served today.
type Blob struct {
	Asset       Asset
	Content     []byte
	Filename    string // suggested name including extension, e.g. "home.svg"
	ContentType string
}

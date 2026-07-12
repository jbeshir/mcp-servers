package assetcore

import (
	"errors"
	"strings"
)

// ErrNotFound is returned by a provider's Fetch (or a routed Fetch given a malformed/unknown id) when
// the requested asset does not exist. It is the provider-neutral counterpart of the provider packages'
// own ErrNotFound values, so callers depend only on assetcore.
var ErrNotFound = errors.New("asset not found")

// License carries the licensing and attribution of an asset. It is a pre-formatted, ready-to-embed
// record (à la Openverse): the Attribution string is copied verbatim into the output manifest, never
// assembled at the call site. Value type; copy freely.
type License struct {
	SPDX                string `json:"spdx"`           // e.g. "CC0-1.0", "OFL-1.1", "MIT"; empty if unknown
	Name                string `json:"name,omitempty"` // human-readable label
	URL                 string `json:"url,omitempty"`  // deed / license text URL
	Attribution         string `json:"attribution"`    // ready-to-embed attribution; empty if none required
	RequiresAttribution bool   `json:"requiresAttribution"`
}

// Asset is a search hit: metadata only, no bytes. Value type; copy freely.
type Asset struct {
	Source     string            // upstream set/collection/family, e.g. "lucide", "open-doodles"
	ID         string            // composite routing key "<provider>:<local>", round-trippable through Fetch
	Kind       Kind              //
	Title      string            // display name within the source (icon/illustration name, font family)
	Tags       []string          //
	License    License           //
	LandingURL string            // upstream page, for credit/debugging
	PreviewURL string            // thumbnail (display only; not the fetch target)
	Meta       map[string]string // display-only metadata surfaced on search hits; never read by Fetch
}

// Well-known Asset.Meta keys. Meta carries per-kind display data that a Search hit wants to show but
// that is not part of the asset's identity or fetch parameters. It is strictly informational: Fetch
// takes typed opts (IconFetchOpts, FontFetchOpts), never Meta.
const (
	MetaCategory = "category" // font family category, for search display
	MetaWeights  = "weights"  // comma-separated font weights, for search display
)

// AssetID composes a provider name and a provider-local id into the canonical composite asset id. The
// local part is opaque to everyone but the emitting provider and may itself contain colons.
func AssetID(provider, local string) string { return provider + ":" + local }

// ParseAssetID splits a composite asset id into its provider name and provider-local part at the first
// colon. ok is true only when both parts are non-empty.
func ParseAssetID(id string) (provider, local string, ok bool) {
	provider, local, _ = strings.Cut(id, ":")
	if provider == "" || local == "" {
		return "", "", false
	}

	return provider, local, true
}

// SearchResult is one provider's page of search hits plus the opaque cursor to its next page.
// NextCursor "" means the provider has no further pages.
type SearchResult struct {
	Assets     []Asset
	NextCursor string
}

// Source describes one upstream set/collection/family a provider serves, for discovery
// (list_asset_sources). Count is the asset count, or -1 if unknown. Meta is optional display data
// (e.g. a font family's category). Value type; copy freely.
type Source struct {
	Name    string
	License License
	Count   int
	Meta    map[string]string
}

// Blob is a materialized asset: bytes plus the metadata the writer layer turns into a file and a
// manifest entry. []byte is sufficient for the small icon/illustration/font payloads served today.
type Blob struct {
	Asset       Asset
	Content     []byte
	Filename    string // suggested name including extension, e.g. "home.svg"
	ContentType string
}

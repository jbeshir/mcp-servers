package assetcore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// encodeCursor packs a provider name -> per-provider cursor map into the opaque aggregate cursor
// token handed back to callers as SearchResult.NextCursor. An empty map encodes to "" so a fully
// exhausted search reports no further pages.
func encodeCursor(next map[string]string) string {
	if len(next) == 0 {
		return ""
	}

	raw, err := json.Marshal(next)
	if err != nil {
		// next is a map[string]string; json.Marshal cannot fail on it.
		panic(fmt.Errorf("assetcore: encode cursor: %w", err))
	}

	return base64.StdEncoding.EncodeToString(raw)
}

// decodeCursor unpacks an aggregate cursor token produced by encodeCursor back into its per-provider
// cursor map. "" decodes to an empty map with a nil error, representing a first-page request.
func decodeCursor(token string) (map[string]string, error) {
	if token == "" {
		return map[string]string{}, nil
	}

	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}

	var next map[string]string
	if err := json.Unmarshal(raw, &next); err != nil {
		return nil, fmt.Errorf("unmarshal cursor: %w", err)
	}

	return next, nil
}

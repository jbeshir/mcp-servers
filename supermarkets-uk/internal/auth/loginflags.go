// Package auth provides authenticated login and cookie persistence for supermarket sessions.
package auth

import (
	"os"
	"strings"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// supermarketEnvPrefixes maps supermarket IDs to their environment variable prefix.
var supermarketEnvPrefixes = map[datasource.SupermarketID]string{
	datasource.Tesco:      "TESCO",
	datasource.Sainsburys: "SAINSBURYS",
	datasource.Ocado:      "OCADO",
	datasource.Morrisons:  "MORRISONS",
	datasource.Asda:       "ASDA",
	datasource.Waitrose:   "WAITROSE",
}

// LoadLoginFlags reads per-supermarket login flags from environment variables.
// A supermarket is included if its <PREFIX>_LOGIN env var is set to a truthy
// value (e.g. "true", "1", "yes").
func LoadLoginFlags() map[datasource.SupermarketID]bool {
	flags := make(map[datasource.SupermarketID]bool)
	for id, prefix := range supermarketEnvPrefixes {
		val := strings.TrimSpace(os.Getenv(prefix + "_LOGIN"))
		val = strings.ToLower(val)
		if val == "true" || val == "1" || val == "yes" {
			flags[id] = true
		}
	}
	return flags
}

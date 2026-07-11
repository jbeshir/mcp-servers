// Package config is the wiring layer for the assets server: it reads configuration from the
// environment (the only place os.Getenv is consulted for new wiring) and constructs the read-only
// assetcore.Registry of providers. Providers never read the environment themselves.
package config

import (
	"os"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedfonts"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedicons"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedillustrations"
)

// envOutputDir names the environment variable selecting the asset output directory.
const envOutputDir = "ASSETS_OUTPUT_DIR"

// Config holds the server's resolved configuration. For this PR the only knob is the output
// directory; no remote-provider or credential settings exist yet.
type Config struct {
	// OutputDir is the ASSETS_OUTPUT_DIR value, surfaced here for logging/wiring. The asset writer
	// (internal/server) still reads this variable itself for the actual write path, so output
	// behaviour is unchanged; centralizing that read is deferred to a later PR.
	OutputDir string
}

// LoadConfig reads the server configuration from the environment.
func LoadConfig() Config {
	return Config{
		OutputDir: os.Getenv(envOutputDir),
	}
}

// Deps are the constructed dependencies the server runs on: the provider registry. The registry is
// treated read-only after construction.
type Deps struct {
	Registry *assetcore.Registry
}

// Setup builds the provider registry from cfg. It registers the three self-contained embedded
// providers unconditionally (the zero-config, offline default); remote providers are out of scope for
// this PR. Each provider owns its own license metadata, so there is no catalog to load.
func Setup(_ Config) *Deps {
	r := assetcore.NewRegistry()
	r.AddIcon(embeddedicons.New())
	r.AddIllustration(embeddedillustrations.New())
	r.AddFont(embeddedfonts.New())

	return &Deps{Registry: r}
}

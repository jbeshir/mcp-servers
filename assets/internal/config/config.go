// Package config is the wiring layer for the assets server: it reads configuration from the
// environment (the only place os.Getenv is consulted for new wiring) and constructs the read-only
// assetcore.Registry of providers. Providers never read the environment themselves.
package config

import (
	"os"
	"path/filepath"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedfonts"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedicons"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedillustrations"
)

// envOutputDir names the environment variable selecting the asset output directory.
const envOutputDir = "ASSETS_OUTPUT_DIR"

// defaultOutputDirName is the subdirectory of the OS temp dir used when ASSETS_OUTPUT_DIR is unset.
const defaultOutputDirName = "assets-mcp"

// Config holds the server's resolved configuration. For this PR the only knob is the output
// directory; no remote-provider or credential settings exist yet.
type Config struct {
	// OutputDir is the raw ASSETS_OUTPUT_DIR value; empty means "use the default temp directory".
	OutputDir string
}

// LoadConfig reads the server configuration from the environment.
func LoadConfig() Config {
	return Config{
		OutputDir: os.Getenv(envOutputDir),
	}
}

// Deps are the constructed dependencies the server runs on: the provider registry and the resolved
// output directory rendered assets are written to.
type Deps struct {
	Registry  *assetcore.Registry
	OutputDir string
}

// Setup builds the provider registry from cfg and resolves the asset output directory. It registers
// the three self-contained embedded providers unconditionally (the zero-config, offline default);
// remote providers are out of scope for this PR. Each provider owns its own license metadata, so there
// is no catalog to load.
func Setup(cfg Config) *Deps {
	r := assetcore.NewRegistry()
	r.AddIcon(embeddedicons.New())
	r.AddIllustration(embeddedillustrations.New())
	r.AddFont(embeddedfonts.New())

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), defaultOutputDirName)
	}

	return &Deps{Registry: r, OutputDir: outputDir}
}

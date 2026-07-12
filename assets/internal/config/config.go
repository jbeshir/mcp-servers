// Package config is the wiring layer for the assets server: it reads configuration from the
// environment (the only place os.Getenv is consulted for new wiring) and constructs the read-only
// assetcore.Registry of providers. Providers never read the environment themselves.
package config

import (
	"os"
	"path/filepath"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/ambientcg"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedfonts"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedicons"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/embeddedillustrations"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/googlefonts"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/iconify"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/openverse"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

// envOutputDir names the environment variable selecting the asset output directory.
const envOutputDir = "ASSETS_OUTPUT_DIR"

// envDisableRemote names the environment variable that, when set to any non-empty value, disables
// the keyless remote providers and runs the server against only the embedded offline base.
const envDisableRemote = "ASSETS_DISABLE_REMOTE"

// envCacheDir names the environment variable selecting the on-disk cache directory used by remote
// providers.
const envCacheDir = "ASSETS_CACHE_DIR"

// defaultOutputDirName is the subdirectory of the OS temp dir used when ASSETS_OUTPUT_DIR is unset.
const defaultOutputDirName = "assets-mcp"

// defaultCacheDirName is the subdirectory of the OS cache (or temp, on fallback) dir used when
// ASSETS_CACHE_DIR is unset.
const defaultCacheDirName = "assets-mcp"

// Remote provider rate limits: requests per second and burst size, chosen to stay comfortably under
// each upstream's documented or observed anonymous-use limits.
const (
	// openverseRPS and openverseBurst are gentle: Openverse caps anonymous callers at ~20 req/min.
	openverseRPS   = 0.3
	openverseBurst = 5

	// remoteRPS and remoteBurst apply to the other keyless remotes (Iconify, Google Fonts,
	// ambientCG), none of which document as tight a limit as Openverse.
	remoteRPS   = 5
	remoteBurst = 5
)

// Config holds the server's resolved configuration: the output directory, and the remote-provider
// toggle and cache directory used when remote providers are enabled.
type Config struct {
	// OutputDir is the raw ASSETS_OUTPUT_DIR value; empty means "use the default temp directory".
	OutputDir string

	// DisableRemote, when true, skips registering the keyless remote providers (iconify,
	// googlefonts, openverse, ambientcg), leaving only the embedded offline base.
	DisableRemote bool

	// CacheDir is the raw ASSETS_CACHE_DIR value; empty means "use the default OS cache directory".
	CacheDir string
}

// LoadConfig reads the server configuration from the environment.
func LoadConfig() Config {
	return Config{
		OutputDir:     os.Getenv(envOutputDir),
		DisableRemote: os.Getenv(envDisableRemote) != "",
		CacheDir:      os.Getenv(envCacheDir),
	}
}

// Deps are the constructed dependencies the server runs on: the provider registry and the resolved
// output directory rendered assets are written to.
type Deps struct {
	Registry  *assetcore.Registry
	OutputDir string
}

// Setup builds the provider registry from cfg and resolves the asset output directory. It registers
// the three self-contained embedded providers unconditionally (the zero-config, offline base), then,
// unless cfg.DisableRemote is set, additionally registers the keyless remote providers (iconify,
// googlefonts, openverse, ambientcg) behind a shared HTTP client and on-disk cache. Each provider owns
// its own license metadata, so there is no catalog to load.
func Setup(cfg Config) *Deps {
	r := assetcore.NewRegistry()
	r.AddIcon(embeddedicons.New())
	r.AddIllustration(embeddedillustrations.New())
	r.AddFont(embeddedfonts.New())

	if !cfg.DisableRemote {
		addRemoteProviders(r, cfg)
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), defaultOutputDirName)
	}

	return &Deps{Registry: r, OutputDir: outputDir}
}

// addRemoteProviders registers the keyless remote providers onto r behind a shared HTTP client and
// on-disk cache, each with its own polite rate limiter.
func addRemoteProviders(r *assetcore.Registry, cfg Config) {
	client := httpx.New(httpx.Config{})
	c := cache.New(resolveCacheDir(cfg))

	r.AddIcon(iconify.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
	r.AddFont(googlefonts.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
	r.AddPhoto(openverse.New(client, ratelimit.New(openverseRPS, openverseBurst), c))
	r.AddTexture(ambientcg.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
}

// resolveCacheDir picks the on-disk directory remote providers cache fetched bytes in: cfg.CacheDir
// when set, else the OS cache directory, falling back to the OS temp directory if that's unavailable.
func resolveCacheDir(cfg Config) string {
	if cfg.CacheDir != "" {
		return cfg.CacheDir
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), defaultCacheDirName)
	}

	return filepath.Join(dir, defaultCacheDirName)
}

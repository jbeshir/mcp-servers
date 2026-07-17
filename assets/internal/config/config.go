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
	"github.com/jbeshir/mcp-servers/assets/internal/providers/pexels"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/pixabay"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/polyhaven"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/polypizza"
	"github.com/jbeshir/mcp-servers/assets/internal/providers/unsplash"
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

// Environment variable names for the opt-in keyed providers: an access key/token for the first four,
// and a plain enable flag for Poly Haven (which is keyless but gated by non-commercial API terms).
const (
	envUnsplashAccessKey = "ASSETS_UNSPLASH_ACCESS_KEY"
	envPixabayKey        = "ASSETS_PIXABAY_KEY"
	envPexelsKey         = "ASSETS_PEXELS_KEY"
	envPolyPizzaKey      = "ASSETS_POLYPIZZA_KEY"
	envPolyHavenEnable   = "ASSETS_POLYHAVEN_ENABLE"
)

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

// Rate limits for the opt-in keyed providers, sized to each upstream's documented ceiling. Each
// provider's pair is wired into its own ratelimit.Limiter, pacing that provider independently of
// the others.
const (
	// unsplashRPS and unsplashBurst: Unsplash's demo tier caps at ~50 req/hr.
	unsplashRPS   = 50.0 / 3600
	unsplashBurst = 5

	// pixabayRPS and pixabayBurst: Pixabay documents ~100 req/min.
	pixabayRPS   = 100.0 / 60
	pixabayBurst = 5

	// pexelsRPS and pexelsBurst: Pexels documents ~200 req/hr.
	pexelsRPS   = 200.0 / 3600
	pexelsBurst = 5

	// polyPizzaRPS and polyPizzaBurst: Poly Pizza documents no limit; stay modest.
	polyPizzaRPS   = 1
	polyPizzaBurst = 3

	// polyHavenRPS and polyHavenBurst: Poly Haven's API is non-commercial use only; stay modest.
	polyHavenRPS   = 1
	polyHavenBurst = 3
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

	// UnsplashAccessKey gates the opt-in Unsplash provider: empty means it is simply not registered,
	// leaving the free providers as the default.
	UnsplashAccessKey string

	// PixabayKey gates the opt-in Pixabay provider: empty means it is simply not registered, leaving
	// the free providers as the default.
	PixabayKey string

	// PexelsKey gates the opt-in Pexels provider: empty means it is simply not registered, leaving the
	// free providers as the default.
	PexelsKey string

	// PolyPizzaKey gates the opt-in Poly Pizza provider: empty means it is simply not registered,
	// leaving the free providers as the default.
	PolyPizzaKey string

	// PolyHavenEnable gates the opt-in Poly Haven provider: unset means it is simply not registered,
	// leaving the free providers as the default.
	PolyHavenEnable bool
}

// LoadConfig reads the server configuration from the environment.
func LoadConfig() Config {
	return Config{
		OutputDir:         os.Getenv(envOutputDir),
		DisableRemote:     os.Getenv(envDisableRemote) != "",
		CacheDir:          os.Getenv(envCacheDir),
		UnsplashAccessKey: os.Getenv(envUnsplashAccessKey),
		PixabayKey:        os.Getenv(envPixabayKey),
		PexelsKey:         os.Getenv(envPexelsKey),
		PolyPizzaKey:      os.Getenv(envPolyPizzaKey),
		PolyHavenEnable:   os.Getenv(envPolyHavenEnable) != "",
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
// googlefonts, openverse, ambientcg) and any opt-in keyed providers whose credential is configured
// (e.g. Unsplash), all behind a shared HTTP client and on-disk cache. Each provider owns its own license
// metadata, so there is no catalog to load.
func Setup(cfg Config) *Deps {
	r := assetcore.NewRegistry()
	r.AddIcon(embeddedicons.New())
	r.AddIllustration(embeddedillustrations.New())
	r.AddFont(embeddedfonts.New())

	if !cfg.DisableRemote {
		client := httpx.New(httpx.Config{})
		c := cache.New(resolveCacheDir(cfg))
		addRemoteProviders(r, client, c)
		addKeyedProviders(r, cfg, client, c)
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), defaultOutputDirName)
	}

	return &Deps{Registry: r, OutputDir: outputDir}
}

// addRemoteProviders registers the keyless remote providers onto r behind the shared HTTP client and
// on-disk cache, each with its own polite rate limiter.
func addRemoteProviders(r *assetcore.Registry, client *httpx.Client, c *cache.Cache) {
	r.AddIcon(iconify.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
	r.AddFont(googlefonts.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
	r.AddPhoto(openverse.New(client, ratelimit.New(openverseRPS, openverseBurst), c))
	r.AddTexture(ambientcg.New(client, ratelimit.New(remoteRPS, remoteBurst), c))
}

// addKeyedProviders registers each opt-in keyed remote provider onto r behind the shared HTTP client
// and on-disk cache, each with its own polite rate limiter. A provider is registered only when its
// corresponding cfg credential (or, for Poly Haven, its enable flag) is set, leaving the free providers
// as the default.
func addKeyedProviders(r *assetcore.Registry, cfg Config, client *httpx.Client, c *cache.Cache) {
	if cfg.UnsplashAccessKey != "" {
		r.AddPhoto(unsplash.New(client, ratelimit.New(unsplashRPS, unsplashBurst), c, cfg.UnsplashAccessKey))
	}
	if cfg.PixabayKey != "" {
		r.AddPhoto(pixabay.New(client, ratelimit.New(pixabayRPS, pixabayBurst), c, cfg.PixabayKey))
	}
	if cfg.PexelsKey != "" {
		r.AddPhoto(pexels.New(client, ratelimit.New(pexelsRPS, pexelsBurst), c, cfg.PexelsKey))
	}
	if cfg.PolyPizzaKey != "" {
		r.AddModel(polypizza.New(client, ratelimit.New(polyPizzaRPS, polyPizzaBurst), c, cfg.PolyPizzaKey))
	}
	if cfg.PolyHavenEnable {
		r.AddModel(polyhaven.New(client, ratelimit.New(polyHavenRPS, polyHavenBurst), c))
	}
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

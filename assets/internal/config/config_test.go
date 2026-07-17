package config

import (
	"os"
	"path/filepath"
	"testing"
)

// providerNames returns the Name() of every provider registered on r, across all kinds.
func providerNames(t *testing.T, deps *Deps) map[string]bool {
	t.Helper()

	names := map[string]bool{}
	for _, info := range deps.Registry.Providers() {
		names[info.Name] = true
	}

	return names
}

func TestLoadConfigHonorsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envOutputDir, dir)

	cfg := LoadConfig()
	if cfg.OutputDir != dir {
		t.Errorf("LoadConfig().OutputDir = %q, want %q", cfg.OutputDir, dir)
	}
}

func TestLoadConfigDefaultsEmpty(t *testing.T) {
	t.Setenv(envOutputDir, "")

	cfg := LoadConfig()
	if cfg.OutputDir != "" {
		t.Errorf("LoadConfig().OutputDir = %q, want empty", cfg.OutputDir)
	}
}

func TestSetupOutputDirHonorsConfig(t *testing.T) {
	dir := t.TempDir()

	deps := Setup(Config{OutputDir: dir})
	if deps.OutputDir != dir {
		t.Errorf("Setup(...).OutputDir = %q, want %q", deps.OutputDir, dir)
	}
}

func TestSetupOutputDirDefault(t *testing.T) {
	deps := Setup(Config{OutputDir: ""})

	want := filepath.Join(os.TempDir(), defaultOutputDirName)
	if deps.OutputDir != want {
		t.Errorf("Setup(...).OutputDir = %q, want %q", deps.OutputDir, want)
	}
}

func TestLoadConfigHonorsDisableRemoteAndCacheDir(t *testing.T) {
	t.Setenv(envDisableRemote, "1")
	t.Setenv(envCacheDir, "/tmp/some-cache")

	cfg := LoadConfig()
	if !cfg.DisableRemote {
		t.Errorf("LoadConfig().DisableRemote = false, want true")
	}
	if cfg.CacheDir != "/tmp/some-cache" {
		t.Errorf("LoadConfig().CacheDir = %q, want %q", cfg.CacheDir, "/tmp/some-cache")
	}
}

func TestLoadConfigDisableRemoteDefaultsFalse(t *testing.T) {
	t.Setenv(envDisableRemote, "")
	t.Setenv(envCacheDir, "")

	cfg := LoadConfig()
	if cfg.DisableRemote {
		t.Errorf("LoadConfig().DisableRemote = true, want false")
	}
	if cfg.CacheDir != "" {
		t.Errorf("LoadConfig().CacheDir = %q, want empty", cfg.CacheDir)
	}
}

func TestLoadConfigHonorsKeyedProviderEnv(t *testing.T) {
	t.Setenv(envUnsplashAccessKey, "unsplash-key")
	t.Setenv(envPixabayKey, "pixabay-key")
	t.Setenv(envPexelsKey, "pexels-key")
	t.Setenv(envPolyPizzaKey, "polypizza-key")
	t.Setenv(envPolyHavenEnable, "1")

	cfg := LoadConfig()
	if cfg.UnsplashAccessKey != "unsplash-key" {
		t.Errorf("LoadConfig().UnsplashAccessKey = %q, want %q", cfg.UnsplashAccessKey, "unsplash-key")
	}
	if cfg.PixabayKey != "pixabay-key" {
		t.Errorf("LoadConfig().PixabayKey = %q, want %q", cfg.PixabayKey, "pixabay-key")
	}
	if cfg.PexelsKey != "pexels-key" {
		t.Errorf("LoadConfig().PexelsKey = %q, want %q", cfg.PexelsKey, "pexels-key")
	}
	if cfg.PolyPizzaKey != "polypizza-key" {
		t.Errorf("LoadConfig().PolyPizzaKey = %q, want %q", cfg.PolyPizzaKey, "polypizza-key")
	}
	if !cfg.PolyHavenEnable {
		t.Errorf("LoadConfig().PolyHavenEnable = false, want true")
	}
}

func TestLoadConfigKeyedProviderEnvDefaultsEmpty(t *testing.T) {
	t.Setenv(envUnsplashAccessKey, "")
	t.Setenv(envPixabayKey, "")
	t.Setenv(envPexelsKey, "")
	t.Setenv(envPolyPizzaKey, "")
	t.Setenv(envPolyHavenEnable, "")

	cfg := LoadConfig()
	if cfg.UnsplashAccessKey != "" {
		t.Errorf("LoadConfig().UnsplashAccessKey = %q, want empty", cfg.UnsplashAccessKey)
	}
	if cfg.PixabayKey != "" {
		t.Errorf("LoadConfig().PixabayKey = %q, want empty", cfg.PixabayKey)
	}
	if cfg.PexelsKey != "" {
		t.Errorf("LoadConfig().PexelsKey = %q, want empty", cfg.PexelsKey)
	}
	if cfg.PolyPizzaKey != "" {
		t.Errorf("LoadConfig().PolyPizzaKey = %q, want empty", cfg.PolyPizzaKey)
	}
	if cfg.PolyHavenEnable {
		t.Errorf("LoadConfig().PolyHavenEnable = true, want false")
	}
}

var embeddedProviderNames = []string{"embedded-icons", "embedded-illustrations", "embedded-fonts"}

var remoteProviderNames = []string{"iconify", "googlefonts", "openverse", "ambientcg"}

func TestSetupAlwaysRegistersEmbeddedProviders(t *testing.T) {
	for _, disableRemote := range []bool{false, true} {
		deps := Setup(Config{DisableRemote: disableRemote})
		names := providerNames(t, deps)

		for _, name := range embeddedProviderNames {
			if !names[name] {
				t.Errorf("DisableRemote=%v: provider %q not registered", disableRemote, name)
			}
		}
	}
}

func TestSetupDisableRemoteOmitsRemoteProviders(t *testing.T) {
	deps := Setup(Config{DisableRemote: true})
	names := providerNames(t, deps)

	for _, name := range remoteProviderNames {
		if names[name] {
			t.Errorf("DisableRemote=true: provider %q should not be registered", name)
		}
	}
}

func TestSetupDefaultRegistersRemoteProviders(t *testing.T) {
	deps := Setup(Config{})
	names := providerNames(t, deps)

	for _, name := range remoteProviderNames {
		if !names[name] {
			t.Errorf("DisableRemote=false: provider %q not registered", name)
		}
	}
}

func TestSetupHonorsExplicitCacheDir(t *testing.T) {
	dir := t.TempDir()

	deps := Setup(Config{CacheDir: filepath.Join(dir, "cache")})
	names := providerNames(t, deps)

	for _, name := range remoteProviderNames {
		if !names[name] {
			t.Errorf("provider %q not registered with explicit CacheDir set", name)
		}
	}
}

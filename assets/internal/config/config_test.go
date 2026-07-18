package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/stretchr/testify/require"
)

func writeAssetsDBFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// #nosec G304 -- the path is a fixed test fixture beneath t.TempDir.
	file, err := os.Create(filepath.Join(dir, "datapackage.json"))
	require.NoError(t, err)
	dataPackage := map[string]any{
		"name": "fixture", "title": "Fixture", "version": "1",
		"created": "2026-07-18T00:00:00Z", "x_assetsdb:schemaVersion": 1,
		"x_assetsdb:sources": []any{map[string]any{"name": "pack", "title": "Pack", "path": "sources/pack.zip", "licenses": []any{map[string]any{"name": "CC0-1.0"}}}},
		"resources": []any{
			map[string]any{"name": "model", "x_assetsdb:id": "assetsdb:pack/model.glb", "x_assetsdb:source": "pack", "x_assetsdb:kind": "model3d", "path": "model.glb"},
			map[string]any{"name": "audio", "x_assetsdb:id": "assetsdb:pack/audio.ogg", "x_assetsdb:source": "pack", "x_assetsdb:kind": "audio", "path": "audio.ogg"},
			map[string]any{"name": "font", "x_assetsdb:id": "assetsdb:pack/font.ttf", "x_assetsdb:source": "pack", "x_assetsdb:kind": "font", "path": "font.ttf"},
			map[string]any{"name": "sprite", "x_assetsdb:id": "assetsdb:pack/sprite.png", "x_assetsdb:source": "pack", "x_assetsdb:kind": "sprite2d", "path": "sprite.png"},
		},
	}
	require.NoError(t, json.NewEncoder(file).Encode(dataPackage))
	require.NoError(t, file.Close())

	return dir
}

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
	t.Setenv(envAssetsDB, "/tmp/assetsdb-fixture")

	cfg := LoadConfig()
	if cfg.OutputDir != dir {
		t.Errorf("LoadConfig().OutputDir = %q, want %q", cfg.OutputDir, dir)
	}
	if cfg.AssetsDB != "/tmp/assetsdb-fixture" {
		t.Errorf("LoadConfig().AssetsDB = %q", cfg.AssetsDB)
	}
}

func TestSetupInvalidAssetsDBIsNonFatal(t *testing.T) {
	deps := Setup(Config{AssetsDB: filepath.Join(t.TempDir(), "missing"), DisableRemote: true})
	require.NotNil(t, deps.Registry)
	require.Nil(t, deps.PackStore)
	require.Empty(t, deps.Registry.Sprites())
}

func TestSetupValidAssetsDBRegistersAllKindsWithRemoteDisabled(t *testing.T) {
	deps := Setup(Config{AssetsDB: writeAssetsDBFixture(t), DisableRemote: true})
	require.NotNil(t, deps.PackStore)
	require.Len(t, deps.Registry.Models(), 1)
	require.Len(t, deps.Registry.Audio(), 1)
	require.Len(t, deps.Registry.Sprites(), 1)
	require.Len(t, deps.Registry.Fonts(), 2) // embedded and assetsdb
	kinds := map[assetcore.Kind]bool{}
	for _, info := range deps.Registry.Providers() {
		if info.Name == "assetsdb" {
			kinds[info.Kind] = true
		}
	}
	for _, kind := range []assetcore.Kind{assetcore.KindModel, assetcore.KindAudio, assetcore.KindFont, assetcore.KindSprite} {
		require.True(t, kinds[kind], "assetsdb provider missing kind %q", kind)
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

var keyedProviderNames = []string{"unsplash", "pixabay", "pexels", "polypizza", "polyhaven"}

func TestSetupRegistersKeyedProvidersWhenCredentialsSet(t *testing.T) {
	deps := Setup(Config{
		UnsplashAccessKey: "k",
		PixabayKey:        "k",
		PexelsKey:         "k",
		PolyPizzaKey:      "k",
		PolyHavenEnable:   true,
	})
	names := providerNames(t, deps)

	for _, name := range keyedProviderNames {
		require.True(t, names[name], "provider %q should be registered when its key/flag is set", name)
	}
}

func TestSetupOmitsKeyedProvidersWithoutCredentials(t *testing.T) {
	deps := Setup(Config{})
	names := providerNames(t, deps)

	for _, name := range keyedProviderNames {
		require.False(t, names[name], "provider %q should not be registered without its key/flag", name)
	}
	require.True(t, names["openverse"], "keyless remote %q should still be registered", "openverse")
	require.True(t, names["ambientcg"], "keyless remote %q should still be registered", "ambientcg")
}

func TestSetupDisableRemoteOmitsKeyedProvidersEvenWithCredentials(t *testing.T) {
	deps := Setup(Config{
		DisableRemote:     true,
		UnsplashAccessKey: "k",
		PixabayKey:        "k",
		PexelsKey:         "k",
		PolyPizzaKey:      "k",
		PolyHavenEnable:   true,
	})
	names := providerNames(t, deps)

	for _, name := range keyedProviderNames {
		require.False(t, names[name], "DisableRemote=true: keyed provider %q should not be registered", name)
	}
	for _, name := range remoteProviderNames {
		require.False(t, names[name], "DisableRemote=true: keyless remote %q should not be registered", name)
	}
	for _, name := range embeddedProviderNames {
		require.True(t, names[name], "DisableRemote=true: embedded provider %q should still be registered", name)
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

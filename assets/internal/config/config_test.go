package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

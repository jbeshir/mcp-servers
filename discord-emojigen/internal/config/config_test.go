package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadDiscordConfigurationAndUnsetAllowedRoots(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("DISCORD_EMOJIGEN_HTTP_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DiscordToken != "token" || cfg.GuildID != "100" || cfg.HTTPTimeout != 5*time.Second {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.AllowedImageRoots != nil {
		t.Fatalf("allowed roots = %#v, want nil", cfg.AllowedImageRoots)
	}
}

func TestLoadNormalizesCanonicalAllowedImageRoots(t *testing.T) {
	setRequiredEnvironment(t)
	first := t.TempDir()
	second := t.TempDir()
	firstLink := filepath.Join(t.TempDir(), "first-link")
	if err := os.Symlink(first, firstLink); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS",
		" "+first+string(os.PathListSeparator)+second+" "+
			string(os.PathListSeparator)+firstLink,
	)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Clean(first), filepath.Clean(second)}
	if !reflect.DeepEqual(cfg.AllowedImageRoots, want) {
		t.Fatalf("allowed roots = %#v, want %#v", cfg.AllowedImageRoots, want)
	}
}

func TestLoadRejectsInvalidAllowedImageRoots(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	emptyEntry := t.TempDir() + string(os.PathListSeparator)
	tests := []struct {
		name string
		raw  string
	}{
		{"empty entry", emptyEntry},
		{"whitespace entry", t.TempDir() + string(os.PathListSeparator) + "   "},
		{"relative", "relative/path"},
		{"nonexistent", missing},
		{"file", file},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS", tt.raw)
			if _, err := Load(); err == nil ||
				!strings.Contains(err.Error(), "DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS") {
				t.Fatalf("error = %v, want sanitized allowed-roots error", err)
			}
		})
	}
}

func TestLoadValidation(t *testing.T) {
	for _, tt := range []struct {
		name, token, guild, timeout string
	}{
		{"missing token", "", "100", ""},
		{"missing guild", "token", "", ""},
		{"bad timeout", "token", "100", "nope"},
		{"nonpositive timeout", "token", "100", "0s"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISCORD_EMOJIGEN_BOT_TOKEN", tt.token)
			t.Setenv("DISCORD_EMOJIGEN_GUILD_ID", tt.guild)
			t.Setenv("DISCORD_EMOJIGEN_HTTP_TIMEOUT", tt.timeout)
			t.Setenv("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS", "")
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DISCORD_EMOJIGEN_BOT_TOKEN", "token")
	t.Setenv("DISCORD_EMOJIGEN_GUILD_ID", "100")
	t.Setenv("DISCORD_EMOJIGEN_HTTP_TIMEOUT", "")
	t.Setenv("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS", "")
}

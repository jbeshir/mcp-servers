package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	DiscordToken      string
	GuildID           string
	HTTPTimeout       time.Duration
	AllowedImageRoots []string
}

func Load() (Config, error) {
	cfg := Config{
		DiscordToken: os.Getenv("DISCORD_EMOJIGEN_BOT_TOKEN"),
		GuildID:      os.Getenv("DISCORD_EMOJIGEN_GUILD_ID"),
		HTTPTimeout:  2 * time.Minute,
	}

	roots, err := loadAllowedImageRoots(os.Getenv("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS"))
	if err != nil {
		return Config{}, err
	}
	cfg.AllowedImageRoots = roots

	if raw := os.Getenv("DISCORD_EMOJIGEN_HTTP_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return Config{}, errors.New("DISCORD_EMOJIGEN_HTTP_TIMEOUT must be a positive duration")
		}
		cfg.HTTPTimeout = d
	}

	switch {
	case cfg.DiscordToken == "":
		return Config{}, errors.New("DISCORD_EMOJIGEN_BOT_TOKEN is required")
	case cfg.GuildID == "":
		return Config{}, errors.New("DISCORD_EMOJIGEN_GUILD_ID is required")
	}
	return cfg, nil
}

func loadAllowedImageRoots(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	roots := make([]string, 0)
	for _, entry := range filepath.SplitList(raw) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS contains an empty entry")
		}
		if !filepath.IsAbs(entry) {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS entries must be absolute directories")
		}
		canonical, err := filepath.EvalSymlinks(filepath.Clean(entry))
		if err != nil {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS entry cannot be resolved")
		}
		info, err := os.Stat(canonical)
		if err != nil {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS entry cannot be accessed")
		}
		if !info.IsDir() {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS entries must be directories")
		}
		canonical, err = filepath.Abs(canonical)
		if err != nil {
			return nil, errors.New("DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS entry cannot be normalized")
		}
		canonical = filepath.Clean(canonical)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		roots = append(roots, canonical)
	}
	return roots, nil
}

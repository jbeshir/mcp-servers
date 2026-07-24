package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

const (
	defaultModel         = "gpt-image-2"
	defaultMaxReferences = 4
	hardMaxReferences    = 8
)

type Config struct {
	DiscordToken  string
	GuildID       string
	OpenAIKey     string
	ImageModel    string
	MaxReferences int
	HTTPTimeout   time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DiscordToken:  os.Getenv("DISCORD_EMOJIGEN_BOT_TOKEN"),
		GuildID:       os.Getenv("DISCORD_EMOJIGEN_GUILD_ID"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		ImageModel:    envOr("DISCORD_EMOJIGEN_IMAGE_MODEL", defaultModel),
		MaxReferences: defaultMaxReferences,
		HTTPTimeout:   2 * time.Minute,
	}

	var err error
	cfg.MaxReferences, err = maxReferences()
	if err != nil {
		return Config{}, err
	}
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
	case cfg.OpenAIKey == "":
		return Config{}, errors.New("OPENAI_API_KEY is required")
	}
	return cfg, nil
}

func maxReferences() (int, error) {
	raw := os.Getenv("DISCORD_EMOJIGEN_MAX_REFERENCES")
	if raw == "" {
		return defaultMaxReferences, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > hardMaxReferences {
		return 0, errors.New("DISCORD_EMOJIGEN_MAX_REFERENCES must be between 0 and 8")
	}
	return n, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

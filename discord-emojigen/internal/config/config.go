package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	DiscordToken string
	GuildID      string
	HTTPTimeout  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DiscordToken: os.Getenv("DISCORD_EMOJIGEN_BOT_TOKEN"),
		GuildID:      os.Getenv("DISCORD_EMOJIGEN_GUILD_ID"),
		HTTPTimeout:  2 * time.Minute,
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
	}
	return cfg, nil
}

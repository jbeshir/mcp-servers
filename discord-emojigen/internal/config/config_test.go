package config

import (
	"testing"
	"time"
)

func TestLoadOnlyDiscordConfiguration(t *testing.T) {
	t.Setenv("DISCORD_EMOJIGEN_BOT_TOKEN", "token")
	t.Setenv("DISCORD_EMOJIGEN_GUILD_ID", "100")
	t.Setenv("DISCORD_EMOJIGEN_HTTP_TIMEOUT", "5s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DiscordToken != "token" || cfg.GuildID != "100" || cfg.HTTPTimeout != 5*time.Second {
		t.Fatalf("unexpected config: %#v", cfg)
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
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

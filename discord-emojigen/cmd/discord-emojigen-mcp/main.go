package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/config"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/generation"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/server"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	emojiService, err := service.New(
		context.Background(),
		discord.NewClient(cfg.DiscordToken, httpClient),
		generation.NewOpenAI(cfg.OpenAIKey, cfg.ImageModel, httpClient),
		cfg.GuildID,
		cfg.MaxReferences,
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.New(emojiService).Run(); err != nil {
		log.Fatal(err)
	}
}

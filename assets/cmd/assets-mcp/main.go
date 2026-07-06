package main

import (
	"log"

	"github.com/jbeshir/mcp-servers/assets/internal/config"
	"github.com/jbeshir/mcp-servers/assets/internal/server"
)

func main() {
	cfg := config.LoadConfig()
	if cfg.OutputDir != "" {
		log.Printf("writing rendered assets to %s", cfg.OutputDir)
	}

	deps := config.Setup(cfg)

	srv := server.NewServer(deps.Registry, deps.Catalog)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

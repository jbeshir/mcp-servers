package main

import (
	"log"
	"os"

	"github.com/jbeshir/mcp-servers/assets/internal/server"
)

func main() {
	if outputDir := os.Getenv("ASSETS_OUTPUT_DIR"); outputDir != "" {
		log.Printf("writing rendered assets to %s", outputDir)
	}

	srv := server.NewServer()

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

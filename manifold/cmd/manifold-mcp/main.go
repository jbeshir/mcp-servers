package main

import (
	"log"
	"os"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/jbeshir/mcp-servers/manifold/internal/server"
)

func main() {
	apiKey := os.Getenv("MANIFOLD_API_KEY")
	if apiKey == "" {
		log.Fatal("MANIFOLD_API_KEY environment variable is required")
	}

	apiURL := os.Getenv("MANIFOLD_API_URL")
	if apiURL == "" {
		apiURL = "https://api.manifold.markets"
	}

	apiClient := client.NewClient(apiURL, apiKey)
	srv := server.NewServer(apiClient)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

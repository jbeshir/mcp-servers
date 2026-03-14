package main

import (
	"log"
	"os"

	"github.com/jbeshir/mcp-servers/wanikani/internal/client"
	"github.com/jbeshir/mcp-servers/wanikani/internal/server"
)

func main() {
	apiKey := os.Getenv("WANIKANI_API_KEY")
	if apiKey == "" {
		log.Fatal("WANIKANI_API_KEY environment variable is required")
	}

	apiURL := os.Getenv("WANIKANI_API_URL")
	if apiURL == "" {
		apiURL = "https://api.wanikani.com"
	}

	apiClient := client.NewClient(apiURL, apiKey)
	srv := server.NewServer(apiClient)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

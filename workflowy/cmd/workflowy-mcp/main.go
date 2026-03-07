package main

import (
	"log"
	"os"
	"time"

	"github.com/jbeshir/mcp-servers/workflowy/internal/cache"
	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
	"github.com/jbeshir/mcp-servers/workflowy/internal/server"
)

func main() {
	apiToken := os.Getenv("WORKFLOWY_API_TOKEN")
	if apiToken == "" {
		log.Fatal("WORKFLOWY_API_TOKEN environment variable is required")
	}

	apiURL := os.Getenv("WORKFLOWY_API_URL")
	if apiURL == "" {
		apiURL = "https://workflowy.com"
	}

	apiClient := client.NewClient(apiURL, apiToken)
	exportCache := cache.NewCache(apiClient.ExportNodes, 60*time.Second)
	srv := server.NewServer(apiClient, exportCache)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

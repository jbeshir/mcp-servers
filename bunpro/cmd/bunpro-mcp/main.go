package main

import (
	"log"
	"os"

	"github.com/jbeshir/mcp-servers/bunpro/internal/auth"
	"github.com/jbeshir/mcp-servers/bunpro/internal/client"
	"github.com/jbeshir/mcp-servers/bunpro/internal/server"
)

func main() {
	apiURL := os.Getenv("BUNPRO_API_URL")
	if apiURL == "" {
		apiURL = "https://api.bunpro.jp"
	}

	loginURL := os.Getenv("BUNPRO_LOGIN_URL")
	if loginURL == "" {
		loginURL = "https://bunpro.jp"
	}

	token := os.Getenv("BUNPRO_API_TOKEN")
	if token == "" {
		email := os.Getenv("BUNPRO_EMAIL")
		password := os.Getenv("BUNPRO_PASSWORD")
		if email == "" || password == "" {
			log.Fatal("Either BUNPRO_API_TOKEN or both BUNPRO_EMAIL and BUNPRO_PASSWORD are required")
		}

		var err error
		token, err = auth.Login(loginURL, email, password)
		if err != nil {
			log.Fatalf("Failed to login to Bunpro: %v", err)
		}
	}

	apiClient := client.NewClient(apiURL, token)
	srv := server.NewServer(apiClient)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

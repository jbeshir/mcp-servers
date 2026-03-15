package main

import (
	"context"
	"log"
	"os"
	"time"

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

	email := os.Getenv("BUNPRO_EMAIL")
	password := os.Getenv("BUNPRO_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("BUNPRO_EMAIL and BUNPRO_PASSWORD environment variables are required")
	}

	loginCtx, loginCancel := context.WithTimeout(context.Background(), 30*time.Second)
	token, err := auth.Login(loginCtx, loginURL, email, password)
	loginCancel()
	if err != nil {
		log.Fatalf("Failed to login to Bunpro: %v", err)
	}

	apiClient := client.NewClient(apiURL, token)
	srv := server.NewServer(apiClient)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

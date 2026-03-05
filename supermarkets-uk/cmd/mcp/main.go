package main

import (
	"log"
	"os"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/server"
)

func main() {
	postcode := os.Getenv("SUPERMARKET_POSTCODE")
	if postcode == "" {
		log.Fatal("SUPERMARKET_POSTCODE environment variable is required")
	}

	c := client.NewClient(postcode)
	srv := server.NewServer(c)

	err := srv.Run()
	c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

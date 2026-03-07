package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/jbeshir/mcp-servers/amazon/internal/scraper"
	"github.com/jbeshir/mcp-servers/amazon/internal/server"
)

func main() {
	regionID, err := lookupRegionID()
	if err != nil {
		log.Fatal(err)
	}

	browser := scraper.NewBrowser()
	srv := server.NewServer(browser, regionID)

	err = srv.Run()
	browser.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func lookupRegionID() (string, error) {
	id := strings.ToLower(os.Getenv("AMAZON_REGION"))
	if id == "" {
		id = "us"
	}
	if _, ok := scraper.Regions[id]; !ok {
		var ids []string
		for k := range scraper.Regions {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		return "", fmt.Errorf(
			"unknown AMAZON_REGION %q; valid regions: %s",
			id, strings.Join(ids, ", "),
		)
	}
	return id, nil
}

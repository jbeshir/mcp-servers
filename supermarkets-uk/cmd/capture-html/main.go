// Command capture-html fetches rendered HTML from supermarket pages using a
// headless browser and saves it to disk for selector discovery.
//
// Usage:
//
//	go run ./cmd/capture-html -store asda -query "milk"
//	go run ./cmd/capture-html -store waitrose -query "bread"
//	go run ./cmd/capture-html -store asda -url "https://www.asda.com/groceries/product/some-id" -wait "h1"
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/scraper"
)

type storeConfig struct {
	searchURL       func(query string) string
	categoryURL     string
	searchWaitSel   string // CSS selector to wait for on search pages
	categoryWaitSel string // CSS selector to wait for on category pages
}

var stores = map[string]storeConfig{
	"asda": {
		searchURL: func(query string) string {
			return "https://www.asda.com/groceries/search/" + url.PathEscape(query)
		},
		categoryURL:     "https://www.asda.com/groceries",
		searchWaitSel:   `a[href^="/groceries/product/"]`,
		categoryWaitSel: `a[href^="/groceries/fruit"]`,
	},
	"tesco": {
		searchURL: func(query string) string {
			return "https://www.tesco.com/groceries/en-GB/search?query=" + url.QueryEscape(query)
		},
		categoryURL:     "https://www.tesco.com/groceries/en-GB/search?query=a",
		searchWaitSel:   `li[data-testid]`,
		categoryWaitSel: `li[data-testid]`,
	},
	"waitrose": {
		searchURL: func(query string) string {
			return "https://www.waitrose.com/ecom/shop/search?searchTerm=" + url.QueryEscape(query)
		},
		categoryURL:     "https://www.waitrose.com/ecom/shop/browse",
		searchWaitSel:   `article[data-testid="product-pod"]`,
		categoryWaitSel: `a[href*="/ecom/shop/browse/groceries/"]`,
	},
}

func main() {
	store := flag.String("store", "", "supermarket to capture (asda, waitrose)")
	query := flag.String("query", "milk", "search query")
	rawURL := flag.String("url", "", "fetch a specific URL instead of search/category pages")
	wait := flag.String("wait", "", "CSS selector to wait for before capturing (for -url mode)")
	outDir := flag.String("out", "", "output directory (default: internal/datasource/<store>/testdata)")
	flag.Parse()

	if *store == "" {
		log.Fatal("-store is required (asda, tesco, or waitrose)")
	}
	cfg, ok := stores[*store]
	if !ok {
		log.Fatalf("unknown store: %s", *store)
	}

	if *outDir == "" {
		*outDir = filepath.Join("internal", "datasource", *store, "testdata")
	}
	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	browser := scraper.NewBrowser()
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *rawURL != "" {
		outFile := filepath.Join(*outDir, "page.html")
		fetch(ctx, browser, *rawURL, outFile, *wait)
		return
	}

	// Fetch search results page.
	searchFile := filepath.Join(*outDir, *store+"_search.html")
	fetch(ctx, browser, cfg.searchURL(*query), searchFile, cfg.searchWaitSel)

	// Fetch category page.
	catFile := filepath.Join(*outDir, *store+"_categories.html")
	fetch(ctx, browser, cfg.categoryURL, catFile, cfg.categoryWaitSel)

	fmt.Println("\nDone. Inspect the saved HTML to find CSS selectors for product containers, titles, prices, etc.")
}

func fetch(ctx context.Context, browser *scraper.Browser, targetURL, outFile, waitSel string) {
	fmt.Printf("Fetching %s ...\n", targetURL)
	if waitSel != "" {
		fmt.Printf("  waiting for: %s\n", waitSel)
	}
	rc, err := browser.Fetch(ctx, targetURL, nil, waitSel)
	if err != nil {
		log.Printf("ERROR fetching %s: %v", targetURL, err)
		return
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("ERROR reading response for %s: %v", targetURL, err)
		return
	}

	if err := os.WriteFile(outFile, data, 0o600); err != nil {
		log.Printf("ERROR writing %s: %v", outFile, err)
		return
	}
	fmt.Printf("  -> saved %s (%d bytes)\n", outFile, len(data))
}

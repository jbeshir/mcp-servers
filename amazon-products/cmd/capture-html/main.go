// Command capture-html fetches rendered HTML from Amazon search pages using a
// headless browser and saves it to disk for selector debugging.
//
// Usage:
//
//	go run ./amazon-products/cmd/capture-html -query "tungsten cube"
//	go run ./amazon-products/cmd/capture-html -query "tungsten cube" -region de
//	go run ./amazon-products/cmd/capture-html -url "https://www.amazon.com/dp/B00XZBIJLS" -wait "#productTitle"
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/jbeshir/mcp-servers/amazon-products/internal/scraper"
)

func main() {
	query := flag.String("query", "", "search query")
	region := flag.String("region", "us", "region ID (e.g. us, uk, de)")
	rawURL := flag.String("url", "", "fetch a specific URL instead of search")
	wait := flag.String("wait", "", "CSS selector to wait for (overrides default)")
	outFile := flag.String("out", "amazon_capture.html", "output file")
	flag.Parse()

	r, ok := scraper.Regions[*region]
	if !ok {
		log.Fatalf("unknown region: %s", *region)
	}

	var targetURL, waitSel string
	switch {
	case *rawURL != "":
		targetURL = *rawURL
		waitSel = *wait
	case *query != "":
		targetURL = r.BaseURL + "/s?" + url.Values{"k": {*query}}.Encode()
		waitSel = *wait
		if waitSel == "" {
			waitSel = `.s-main-slot`
		}
	default:
		log.Fatal("either -query or -url is required")
	}

	browser := scraper.NewBrowser()

	rc, err := fetchWithRetry(browser, targetURL, waitSel)
	if err != nil {
		browser.Close()
		log.Fatal(err)
	}

	data, err := io.ReadAll(rc)
	_ = rc.Close()
	browser.Close()
	if err != nil {
		log.Fatalf("reading response: %v", err)
	}

	if err := os.WriteFile(*outFile, data, 0o600); err != nil {
		log.Fatalf("writing %s: %v", *outFile, err)
	}
	fmt.Printf("  -> saved %s (%d bytes)\n", *outFile, len(data))
}

func fetchWithRetry(browser *scraper.Browser, targetURL, waitSel string) (io.ReadCloser, error) {
	fmt.Printf("Fetching %s\n", targetURL)
	fmt.Printf("  wait selector: %q\n", waitSel)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rc, err := browser.Fetch(ctx, targetURL, waitSel)
	if err == nil {
		return rc, nil
	}

	fmt.Printf("  ERROR: %v\n", err)
	fmt.Println("  Retrying with no wait selector to capture raw page...")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	return browser.Fetch(ctx2, targetURL, "")
}

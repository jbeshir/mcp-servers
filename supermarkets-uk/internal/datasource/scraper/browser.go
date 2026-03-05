package scraper

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const browserRenderTimeout = 30 * time.Second

// Browser manages a shared headless Chrome instance for rendering JS-heavy pages.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	mu      sync.Mutex
	started bool
}

// NewBrowser creates a new Browser. Call Close when done.
func NewBrowser() *Browser {
	return &Browser{}
}

// start lazily initialises the browser allocator on first use.
func (b *Browser) start() {
	if b.started {
		return
	}
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserAgent(
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) "+
				"Chrome/120.0.0.0 Safari/537.36",
		),
	)
	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(
		context.Background(), opts...,
	)
	b.started = true
}

// Fetch navigates to the given URL in a new browser tab, waits for the
// page to render, and returns the rendered HTML as an io.ReadCloser.
// If waitSelector is non-empty, the browser waits for that CSS selector
// to become visible before capturing the HTML.
func (b *Browser) Fetch(
	ctx context.Context, targetURL string, waitSelector ...string,
) (io.ReadCloser, error) {
	b.mu.Lock()
	b.start()
	b.mu.Unlock()

	tabCtx, tabCancel := chromedp.NewContext(b.allocCtx)
	defer tabCancel()

	renderCtx, renderCancel := context.WithTimeout(
		tabCtx, browserRenderTimeout,
	)
	defer renderCancel()

	// Also respect the caller's context for cancellation.
	go func() {
		select {
		case <-ctx.Done():
			renderCancel()
		case <-renderCtx.Done():
		}
	}()

	actions := []chromedp.Action{
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	}

	if len(waitSelector) > 0 && waitSelector[0] != "" {
		actions = append(actions,
			chromedp.WaitVisible(waitSelector[0], chromedp.ByQuery),
		)
	} else {
		actions = append(actions, chromedp.Sleep(2*time.Second))
	}

	var htmlContent string
	actions = append(actions,
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)

	if err := chromedp.Run(renderCtx, actions...); err != nil {
		return nil, fmt.Errorf("browser render %s: %w", targetURL, err)
	}

	return io.NopCloser(strings.NewReader(htmlContent)), nil
}

// Close shuts down the browser.
func (b *Browser) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started && b.allocCancel != nil {
		b.allocCancel()
	}
}

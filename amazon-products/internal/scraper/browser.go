package scraper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const browserRenderTimeout = 30 * time.Second

// Browser manages a shared headless Chrome instance for rendering pages.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	browserCtx    context.Context
	browserCancel context.CancelFunc

	mu      sync.Mutex
	started bool
}

// NewBrowser creates a new Browser. Call Close when done.
func NewBrowser() *Browser {
	return &Browser{}
}

func (b *Browser) start() {
	if b.started {
		return
	}
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// Realistic window size — default headless size is detectable.
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(
			"Mozilla/5.0 (X11; Linux x86_64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) "+
				"Chrome/145.0.0.0 Safari/537.36",
		),
	)
	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(
		context.Background(), opts...,
	)
	b.browserCtx, b.browserCancel = chromedp.NewContext(b.allocCtx)
	b.started = true
}

// Fetch navigates to the given URL in a new browser tab and returns the
// rendered HTML. If waitSelector is non-empty, the browser waits for that
// CSS selector to appear before capturing. Otherwise it waits for the page
// to fully load and for the DOM to stabilise.
//
// When Amazon's WAF returns a 503 challenge page, the browser waits for
// the challenge JS to set cookies, then retries the navigation.
func (b *Browser) Fetch(ctx context.Context, targetURL string, waitSelector string) (io.ReadCloser, error) {
	const maxAttempts = 3

	b.mu.Lock()
	b.start()
	b.mu.Unlock()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		rc, err := b.fetchOnce(ctx, targetURL, waitSelector)
		if err == nil {
			return rc, nil
		}
		lastErr = err
		if !errors.Is(err, ErrBlocked) || attempt == maxAttempts {
			break
		}
		log.Printf("amazon: got blocked on %s (attempt %d/%d), waiting for WAF challenge then retrying",
			targetURL, attempt, maxAttempts)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return nil, lastErr
}

func (b *Browser) fetchOnce(ctx context.Context, targetURL string, waitSelector string) (io.ReadCloser, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("parse target URL: %w", err)
	}
	homepage := u.Scheme + "://" + u.Host + "/"

	tabCtx, tabCancel := chromedp.NewContext(b.browserCtx)
	defer tabCancel()

	renderCtx, renderCancel := context.WithTimeout(tabCtx, browserRenderTimeout)
	defer renderCancel()

	go func() {
		select {
		case <-ctx.Done():
			renderCancel()
		case <-renderCtx.Done():
		}
	}()

	var actions []chromedp.Action

	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(`
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
			Object.defineProperty(navigator, 'plugins', {
				get: () => [1, 2, 3, 4, 5],
			});
			Object.defineProperty(navigator, 'languages', {
				get: () => ['en-US', 'en'],
			});
			window.chrome = {runtime: {}};
			const originalQuery = window.navigator.permissions.query;
			window.navigator.permissions.query = (parameters) =>
				parameters.name === 'notifications'
					? Promise.resolve({state: Notification.permission})
					: originalQuery(parameters);
		`).Do(ctx)
		return err
	}))

	fetchStart := time.Now()

	// Capture the HTTP status code of the most recent document response.
	var statusCode int64
	chromedp.ListenTarget(renderCtx, func(ev interface{}) {
		if resp, ok := ev.(*network.EventResponseReceived); ok {
			if resp.Type == network.ResourceTypeDocument {
				statusCode = resp.Response.Status
			}
		}
	})

	// Navigate to homepage first so WAF challenge cookies get set in
	// this tab before we navigate to the actual target URL.
	actions = append(actions,
		chromedp.Navigate(homepage),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.ActionFunc(waitForPageComplete),
		chromedp.Sleep(2*time.Second),
	)

	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		log.Printf("amazon: homepage loaded for %s (status=%d, %.1fs)",
			homepage, statusCode, time.Since(fetchStart).Seconds())
		// Reset status code before navigating to the target URL.
		statusCode = 0
		return nil
	}))

	// Now navigate to the target URL within the same tab.
	actions = append(actions,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)

	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		log.Printf("amazon: page body ready for %s (status=%d, %.1fs)",
			targetURL, statusCode, time.Since(fetchStart).Seconds())
		return nil
	}))

	// After body is ready, check for error/block pages (fail fast),
	// then wait for the target selector or DOM stability.
	actions = append(actions, chromedp.ActionFunc(waitForPageComplete))
	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		return checkForErrorPage(ctx, statusCode)
	}))

	if waitSelector != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			return waitForSelector(ctx, waitSelector, targetURL, fetchStart)
		}))
	} else {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			return waitForDOMStable(ctx, targetURL, fetchStart)
		}))
	}

	var htmlContent string
	actions = append(actions,
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)

	if err := chromedp.Run(renderCtx, actions...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logPageState(b, targetURL, waitSelector, fetchStart)
		}
		return nil, fmt.Errorf("browser render %s: %w", targetURL, err)
	}

	return io.NopCloser(strings.NewReader(htmlContent)), nil
}

// Sentinel errors for Amazon page states.
var (
	// ErrCAPTCHA indicates Amazon presented a CAPTCHA challenge.
	ErrCAPTCHA = errors.New("amazon returned a CAPTCHA page")
	// ErrBlocked indicates Amazon returned an error page (e.g. 503).
	ErrBlocked = errors.New("amazon returned an error page")
)

// checkForErrorPage detects Amazon error/block pages and returns a
// descriptive error immediately rather than waiting for a selector timeout.
func checkForErrorPage(ctx context.Context, statusCode int64) error {
	var result struct {
		Captcha bool   `json:"captcha"`
		Title   string `json:"title"`
	}
	if err := chromedp.Evaluate(`({
		captcha: document.querySelector('#captchacharacters') !== null
			|| document.querySelector('form[action*="validateCaptcha"]') !== null,
		title: document.title,
	})`, &result).Do(ctx); err != nil {
		return err
	}
	if result.Captcha {
		return ErrCAPTCHA
	}
	if statusCode >= 400 {
		return fmt.Errorf("%w: HTTP %d — %s", ErrBlocked, statusCode, result.Title)
	}
	return nil
}

func waitForSelector(ctx context.Context, sel, targetURL string, fetchStart time.Time) error {
	const interval = 200 * time.Millisecond
	for {
		var found bool
		if err := chromedp.Evaluate(
			fmt.Sprintf(`document.querySelector(%q) !== null`, sel),
			&found,
		).Do(ctx); err != nil {
			return err
		}
		if found {
			log.Printf("amazon: selector %q found for %s (%.1fs)", sel, targetURL, time.Since(fetchStart).Seconds())
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// waitForDOMStable waits until the page's body content stops changing,
// indicating that JavaScript rendering is complete.
func waitForDOMStable(ctx context.Context, targetURL string, fetchStart time.Time) error {
	const (
		pollInterval    = 500 * time.Millisecond
		stableThreshold = 2 // consecutive polls with same length
	)
	var lastLen int
	stableCount := 0
	for {
		var bodyLen int
		if err := chromedp.Evaluate(
			`document.body ? document.body.innerHTML.length : 0`,
			&bodyLen,
		).Do(ctx); err != nil {
			return err
		}
		if bodyLen == lastLen && bodyLen > 0 {
			stableCount++
			if stableCount >= stableThreshold {
				log.Printf("amazon: DOM stable for %s (%.1fs, %d bytes)",
					targetURL, time.Since(fetchStart).Seconds(), bodyLen)
				return nil
			}
		} else {
			stableCount = 0
		}
		lastLen = bodyLen
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// waitForPageComplete polls until document.readyState is "complete",
// meaning the page and all subresources (scripts, stylesheets) have loaded.
func waitForPageComplete(ctx context.Context) error {
	const interval = 200 * time.Millisecond
	for {
		var complete bool
		if err := chromedp.Evaluate(
			`document.readyState === 'complete'`,
			&complete,
		).Do(ctx); err != nil {
			return err
		}
		if complete {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// logPageState captures diagnostic info when a page load times out.
// It opens a fresh tab to navigate to the URL since the original tab's
// CDP session is dead after a timeout.
func logPageState(b *Browser, targetURL, waitSelector string, fetchStart time.Time) {
	diagTab, diagTabCancel := chromedp.NewContext(b.browserCtx)
	defer diagTabCancel()

	diagCtx, diagCancel := context.WithTimeout(diagTab, 10*time.Second)
	defer diagCancel()

	var diag struct {
		URL        string `json:"url"`
		ReadyState string `json:"readyState"`
		Title      string `json:"title"`
		BodyLen    int    `json:"bodyLen"`
		HasSearch  bool   `json:"hasSearch"`
		HasSlot    bool   `json:"hasSlot"`
		HasResult  bool   `json:"hasResult"`
		HasCaptcha bool   `json:"hasCaptcha"`
	}
	err := chromedp.Run(diagCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.ActionFunc(waitForPageComplete),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`({
			url: location.href,
			readyState: document.readyState,
			title: document.title,
			bodyLen: (document.body && document.body.innerHTML.length) || 0,
			hasSearch: document.querySelector('#search') !== null,
			hasSlot: document.querySelector('.s-main-slot') !== null,
			hasResult: document.querySelector('[data-component-type="s-search-result"]') !== null,
			hasCaptcha: document.querySelector('#captchacharacters') !== null
				|| document.querySelector('form[action*="validateCaptcha"]') !== null,
		})`, &diag),
	)
	if err != nil {
		log.Printf("amazon: timeout for %s after %.1fs (diagnostics failed: %v)",
			targetURL, time.Since(fetchStart).Seconds(), err)
		return
	}
	log.Printf("amazon: timeout for %s after %.1fs — "+
		"currentURL=%s readyState=%s title=%q bodyLen=%d "+
		"hasSearch=%v hasSlot=%v hasResult=%v hasCaptcha=%v selector=%q",
		targetURL, time.Since(fetchStart).Seconds(),
		diag.URL, diag.ReadyState, diag.Title, diag.BodyLen,
		diag.HasSearch, diag.HasSlot, diag.HasResult, diag.HasCaptcha, waitSelector)
}

// Close shuts down the browser.
func (b *Browser) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		b.browserCancel()
		b.allocCancel()
	}
}

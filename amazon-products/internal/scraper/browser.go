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

// tabCloseTimeout bounds how long we wait for a tab's graceful CDP close
// before treating the shared browser as poisoned and force-killing it.
// chromedp's own per-action cleanup timeouts are ~1s each, so anything beyond
// this means the graceful close is genuinely wedged rather than merely slow
// (see chromedp issues #866 and #1544). It is a var, not a const, so tests can
// shrink it.
var tabCloseTimeout = 5 * time.Second

// Browser manages a shared headless Chrome instance for rendering pages.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	browserCtx    context.Context
	browserCancel context.CancelFunc

	mu      sync.Mutex
	started bool
	// epoch identifies the current browser generation. It is bumped every time
	// start() launches a fresh process, so a recycle request can tell whether
	// the generation it observed is still the live one.
	epoch uint64
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
	b.epoch++
}

// closeTabBounded runs tabCancel (the per-tab cancel returned by
// chromedp.NewContext) in a background goroutine and races it against
// tabCloseTimeout. tabCancel performs a graceful CDP-level tab close that can
// hang forever if the browser's websocket transport is wedged (it blocks in
// sync.WaitGroup.Wait waiting for a read goroutine that is parked in a
// never-returning socket read). If the graceful close does not finish in time,
// we force-kill the whole browser process via the allocator cancel and reset
// the Browser so the next Fetch launches a fresh process. reqURL identifies the
// request that triggered recovery; epoch identifies the browser generation to
// recycle.
func (b *Browser) closeTabBounded(tabCancel context.CancelFunc, reqURL string, epoch uint64) {
	done := make(chan struct{})
	go func() {
		tabCancel()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(tabCloseTimeout):
		log.Printf("amazon: tab close hung after %s for %s — "+
			"force-killing browser process and recycling", tabCloseTimeout, reqURL)
		b.recycle(epoch)
	}
}

// recycle force-kills the browser process for the given generation and resets
// the Browser so the next Fetch launches a fresh process. It is safe to call
// concurrently and is idempotent per generation: only the first caller for a
// live generation performs the kill; callers for an already-recycled or
// already-replaced generation are no-ops.
//
// The allocator cancel is invoked OUTSIDE the mutex. It SIGKILLs the Chrome OS
// process (via exec.CommandContext's default cmd.Cancel) — an OS-level, bounded
// operation that does not depend on CDP/websocket responsiveness — and thereby
// also unblocks the leaked graceful-close goroutine by tearing down the
// transport it is parked on. Holding the mutex only to flip state (never across
// the cancel) means recycle can neither deadlock with an in-flight Fetch nor
// block a concurrent start().
func (b *Browser) recycle(epoch uint64) {
	b.mu.Lock()
	if !b.started || b.epoch != epoch {
		b.mu.Unlock()
		return
	}
	b.started = false
	allocCancel := b.allocCancel
	b.mu.Unlock()

	allocCancel()
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

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// (Re)ensure a live browser generation on each attempt: a prior
		// attempt whose tab close hung will have recycled the process, so we
		// must relaunch and observe the current epoch before fetching again.
		// Capture epoch and browserCtx together under the lock so the tab we
		// open and the generation we would recycle belong to the same process,
		// even though mcp-go's stdio server dispatches tool calls concurrently
		// across a worker pool onto this shared Browser.
		b.mu.Lock()
		b.start()
		epoch := b.epoch
		browserCtx := b.browserCtx
		b.mu.Unlock()

		rc, err := b.fetchOnce(ctx, browserCtx, targetURL, waitSelector, epoch)
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

func (b *Browser) fetchOnce(
	ctx, browserCtx context.Context, targetURL string, waitSelector string, epoch uint64,
) (io.ReadCloser, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("parse target URL: %w", err)
	}
	homepage := u.Scheme + "://" + u.Host + "/"

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer b.closeTabBounded(tabCancel, targetURL, epoch)

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
			logPageState(b, browserCtx, targetURL, waitSelector, fetchStart, epoch)
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
func logPageState(
	b *Browser, browserCtx context.Context,
	targetURL, waitSelector string, fetchStart time.Time, epoch uint64,
) {
	diagTab, diagTabCancel := chromedp.NewContext(browserCtx)
	defer b.closeTabBounded(diagTabCancel, targetURL, epoch)

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
//
// It force-kills the Chrome process via the allocator cancel FIRST (SIGKILL,
// bounded, independent of CDP), then runs the graceful root-context cancel.
// The original order (browserCancel before allocCancel) could hang forever
// holding b.mu on a wedged transport — the same graceful-close hazard this
// change eliminates elsewhere. With the process already killed, browserCancel's
// internal wait returns immediately, so Close is always bounded.
func (b *Browser) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.started {
		return
	}
	b.started = false
	b.allocCancel()
	b.browserCancel()
}

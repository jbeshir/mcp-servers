package scraper

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"golang.org/x/time/rate"
)

const browserRenderTimeout = 30 * time.Second

// tabCloseTimeout bounds how long we wait for a tab's graceful CDP close
// before treating the shared browser as poisoned and force-killing it.
// chromedp's own per-action cleanup timeouts are ~1s each, so anything beyond
// this means the graceful close is genuinely wedged rather than merely slow
// (see chromedp issues #866 and #1544). It is a var, not a const, so tests can
// shrink it.
var tabCloseTimeout = 5 * time.Second

// Browser manages a shared headless Chrome instance for rendering JS-heavy pages.
// A single browser context is shared across all Fetch calls so that cookies
// set by one page persist for subsequent requests.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	// browserCtx is a shared browser context; new tabs are created within it
	// so that cookies persist between Fetch calls.
	browserCtx    context.Context
	browserCancel context.CancelFunc

	mu      sync.Mutex
	started bool
	// epoch identifies the current browser generation. It is bumped every time
	// start() launches a fresh process, so a recycle request can tell whether
	// the generation it observed is still the live one.
	epoch uint64

	hostLimiters map[string]*rate.Limiter
	limiterMu    sync.Mutex
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
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent(
			"Mozilla/5.0 (X11; Linux x86_64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) "+
				"Chrome/145.0.0.0 Safari/537.36",
		),
	)
	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(
		context.Background(), opts...,
	)
	// Create a shared browser context so cookies persist across tabs.
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
//
// The shared Browser is used concurrently by SearchAll's per-store goroutines,
// so this must be safe under concurrent Fetch calls: see recycle.
func (b *Browser) closeTabBounded(tabCancel context.CancelFunc, reqURL string, epoch uint64) {
	done := make(chan struct{})
	go func() {
		tabCancel()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(tabCloseTimeout):
		log.Printf("supermarkets-uk: tab close hung after %s for %s — "+
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
// block a concurrent start(). A concurrent fetch that was mid-render on the
// killed process gets a transport error from chromedp.Run and fails that one
// request; the next Fetch transparently relaunches a fresh process.
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

// Fetch navigates to the given URL in a new browser tab, waits for the
// page to render, and returns the rendered HTML as an io.ReadCloser.
// If cookies is non-empty, they are set on the browser before navigation.
// If waitSelector is non-empty, the browser waits for that CSS selector
// to become visible before capturing the HTML.
func (b *Browser) Fetch(
	ctx context.Context, targetURL string, cookies []*http.Cookie, waitSelector ...string,
) (io.ReadCloser, error) {
	sel := ""
	if len(waitSelector) > 0 {
		sel = waitSelector[0]
	}
	body, _, err := b.FetchAndReadCookie(ctx, targetURL, cookies, "", sel)
	return body, err
}

// waitForSelector polls via JavaScript until the CSS selector matches an
// element in the DOM. This is more reliable than chromedp.WaitReady for
// pages that do client-side navigation after the initial load.
func waitForSelector(ctx context.Context, sel string) error {
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
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// limiterForHost returns a per-host rate limiter, creating one lazily at 1 req/sec.
func (b *Browser) limiterForHost(rawURL string) *rate.Limiter {
	u, err := url.Parse(rawURL)
	host := rawURL
	if err == nil {
		host = u.Host
	}

	b.limiterMu.Lock()
	defer b.limiterMu.Unlock()
	if b.hostLimiters == nil {
		b.hostLimiters = make(map[string]*rate.Limiter)
	}
	if lim, ok := b.hostLimiters[host]; ok {
		return lim
	}
	lim := rate.NewLimiter(1, 1)
	b.hostLimiters[host] = lim
	return lim
}

// FetchAndReadCookie navigates to the given URL, waits for the page to
// render, and returns both the HTML and the value of a specific cookie.
// This is useful when the browser's JavaScript refreshes tokens that
// are needed for subsequent API calls.
func (b *Browser) FetchAndReadCookie(
	ctx context.Context,
	targetURL string,
	cookies []*http.Cookie,
	cookieName string,
	waitSelector string,
) (io.ReadCloser, string, error) {
	if err := b.limiterForHost(targetURL).Wait(ctx); err != nil {
		return nil, "", err
	}

	// Capture epoch and browserCtx together under the lock so the tab we open
	// and the generation we would recycle belong to the same browser process,
	// even if a concurrent fetch recycles and relaunches in between.
	b.mu.Lock()
	b.start()
	epoch := b.epoch
	browserCtx := b.browserCtx
	b.mu.Unlock()

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer b.closeTabBounded(tabCancel, targetURL, epoch)

	renderCtx, renderCancel := context.WithTimeout(
		tabCtx, browserRenderTimeout,
	)
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
		_, err := page.AddScriptToEvaluateOnNewDocument(
			`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`,
		).Do(ctx)
		return err
	}))
	for _, c := range cookies {
		actions = append(actions,
			network.SetCookie(c.Name, c.Value).
				WithDomain(c.Domain).
				WithPath(c.Path),
		)
	}
	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		u, err := url.Parse(targetURL)
		if err != nil {
			return err
		}
		referrer := u.Scheme + "://" + u.Host + "/"
		headers := network.Headers{"Referer": referrer}
		return network.SetExtraHTTPHeaders(headers).Do(ctx)
	}))
	actions = append(actions,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if waitSelector != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			return waitForSelector(ctx, waitSelector)
		}))
	} else {
		actions = append(actions, chromedp.Sleep(2*time.Second))
	}

	var htmlContent string
	var cookieValue string
	actions = append(actions,
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	// Read cookies from the browser after the page has rendered
	// (and potentially refreshed tokens).
	if cookieName != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			for _, c := range cookies {
				if c.Name == cookieName {
					cookieValue = c.Value
					return nil
				}
			}
			return nil
		}))
	}

	if err := chromedp.Run(renderCtx, actions...); err != nil {
		return nil, "", fmt.Errorf(
			"browser render %s: %w", targetURL, err,
		)
	}

	return io.NopCloser(strings.NewReader(htmlContent)), cookieValue, nil
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

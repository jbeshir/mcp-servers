package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

const loginTimeout = 10 * time.Minute

// LoginConfig defines the login page structure for a supermarket.
type LoginConfig struct {
	LoginURL      string
	Domain        string
	SessionCookie string // Cookie name that indicates a valid session.
	SuccessQuery  string // CSS selector that indicates logged-in state.
	SuccessText   string // Text content that indicates logged-in state.
}

// SupermarketLoginConfigs maps supermarket IDs to their login page configurations.
var SupermarketLoginConfigs = map[datasource.SupermarketID]LoginConfig{
	datasource.Tesco: {
		LoginURL:      "https://secure.tesco.com/account/en-GB/login",
		Domain:        ".tesco.com",
		SessionCookie: "OAuth.AccessToken",
	},
	datasource.Sainsburys: {
		LoginURL:      "https://www.sainsburys.co.uk/gol-ui/oauth/login",
		Domain:        ".sainsburys.co.uk",
		SessionCookie: "WC_PERSISTENT",
	},
	datasource.Ocado: {
		LoginURL:     "https://www.ocado.com/login",
		Domain:       ".ocado.com",
		SuccessQuery: `a[data-test="logout-button"]`,
	},
	datasource.Morrisons: {
		LoginURL:     "https://groceries.morrisons.com/login",
		Domain:       ".morrisons.com",
		SuccessQuery: `a[data-test="logout-button"]`,
	},
	datasource.Asda: {
		LoginURL:     "https://www.asda.com/account",
		Domain:       ".asda.com",
		SuccessText:  "Sign in details",
	},
	datasource.Waitrose: {
		LoginURL:     "https://www.waitrose.com/ecom/login",
		Domain:       ".waitrose.com",
		SuccessQuery: `a[data-test="signOut"]`,
	},
}

// InteractiveLogin opens a visible browser window to the login page and waits
// for the user to complete login manually. Returns session cookies.
func InteractiveLogin(
	ctx context.Context,
	id datasource.SupermarketID,
	cfg LoginConfig,
) ([]*http.Cookie, error) {
	// Build options from defaults but remove automation-revealing flags
	// that trigger bot detection on sites like Tesco.
	var opts []chromedp.ExecAllocatorOption
	for _, o := range chromedp.DefaultExecAllocatorOptions {
		opts = append(opts, o)
	}
	opts = append(opts,
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	loginCtx, loginCancel := context.WithTimeout(tabCtx, loginTimeout)
	defer loginCancel()

	// Navigate to the login page; the user handles everything.
	if err := chromedp.Run(loginCtx,
		chromedp.Navigate(cfg.LoginURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("login to %s: navigate: %w", id, err)
	}

	// Poll until login is complete.
	var jsCheck string
	switch {
	case cfg.SessionCookie != "":
		if err := pollForCookie(loginCtx, id, cfg); err != nil {
			return nil, err
		}
	case cfg.SuccessQuery != "":
		jsCheck = fmt.Sprintf(
			`document.querySelector(%q) !== null`,
			cfg.SuccessQuery,
		)
	case cfg.SuccessText != "":
		jsCheck = fmt.Sprintf(
			`document.body.innerText.includes(%q)`,
			cfg.SuccessText,
		)
	}
	if jsCheck != "" {
		if err := pollForJS(loginCtx, id, jsCheck); err != nil {
			return nil, err
		}
	}

	return extractCookies(loginCtx, id, cfg)
}

// pollForJS evaluates a JS expression every 500ms until it returns true.
// Unlike PollFunction, this survives page navigations that destroy the
// execution context.
func pollForJS(
	ctx context.Context,
	id datasource.SupermarketID,
	jsExpr string,
) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"login to %s: timed out waiting for page condition: %w",
				id, ctx.Err(),
			)
		case <-ticker.C:
			var result bool
			err := chromedp.Run(ctx,
				chromedp.Evaluate(jsExpr, &result),
			)
			if err != nil {
				// Context destroyed during navigation — retry.
				continue
			}
			if result {
				return nil
			}
		}
	}
}

// pollForCookie polls the browser's cookies until the session cookie appears.
func pollForCookie(
	ctx context.Context,
	id datasource.SupermarketID,
	cfg LoginConfig,
) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"login to %s: timed out waiting for %s cookie: %w",
				id, cfg.SessionCookie, ctx.Err(),
			)
		case <-ticker.C:
			var cdpCookies []*network.Cookie
			if err := chromedp.Run(ctx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					var err error
					cdpCookies, err = network.GetCookies().Do(ctx)
					return err
				}),
			); err != nil {
				return fmt.Errorf(
					"login to %s: check cookies: %w", id, err,
				)
			}
			for _, c := range cdpCookies {
				if c.Name == cfg.SessionCookie {
					return nil
				}
			}
		}
	}
}

// extractCookies retrieves all cookies from the browser and filters by domain.
func extractCookies(
	ctx context.Context,
	id datasource.SupermarketID,
	cfg LoginConfig,
) ([]*http.Cookie, error) {
	var cdpCookies []*network.Cookie
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cdpCookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("login to %s: extract cookies: %w", id, err)
	}

	var cookies []*http.Cookie
	for _, c := range cdpCookies {
		if !strings.HasSuffix(c.Domain, cfg.Domain) &&
			c.Domain != strings.TrimPrefix(cfg.Domain, ".") {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    sanitizeCookieValue(c.Value),
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Unix(int64(c.Expires), 0),
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	if len(cookies) == 0 {
		return nil, fmt.Errorf(
			"login to %s: no cookies found for domain %s",
			id, cfg.Domain,
		)
	}

	return cookies, nil
}

// sanitizeCookieValue removes characters that are invalid in HTTP cookie
// values (e.g. double quotes) to prevent net/http warnings.
func sanitizeCookieValue(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == ' ' || r == ',' || r == ';' || r == '\\' {
			return -1
		}
		return r
	}, v)
}

// HasSessionCookie checks whether a cookie slice contains the expected
// session cookie for the given supermarket. Returns true if no session
// cookie is configured (i.e. any cookies are acceptable).
func HasSessionCookie(
	id datasource.SupermarketID,
	cookies []*http.Cookie,
) bool {
	cfg, ok := SupermarketLoginConfigs[id]
	if !ok || cfg.SessionCookie == "" {
		return true
	}
	for _, c := range cookies {
		if c.Name == cfg.SessionCookie {
			return true
		}
	}
	return false
}

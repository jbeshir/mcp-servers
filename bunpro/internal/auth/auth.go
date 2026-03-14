package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

var csrfPattern = regexp.MustCompile(
	`name="authenticity_token"\s+value="([^"]+)"`,
)

const browserUA = "Mozilla/5.0 (X11; Linux x86_64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/131.0.0.0 Safari/537.36"

// Login performs the Devise login flow and returns the frontend_api_token.
//
// The flow is:
//  1. GET /login to obtain a CSRF token and session cookie
//  2. POST /users/sign_in with form-encoded credentials and CSRF token
//  3. Extract the frontend_api_token cookie from the response
func Login(baseURL, email, password string) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("creating cookie jar: %w", err)
	}
	httpClient := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ctx := context.Background()

	csrf, err := fetchCSRF(ctx, httpClient, baseURL)
	if err != nil {
		return "", err
	}

	token, err := submitLogin(ctx, httpClient, baseURL, email, password, csrf)
	if err != nil {
		return "", err
	}
	return token, nil
}

func fetchCSRF(ctx context.Context, httpClient *http.Client, baseURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/login", nil)
	if err != nil {
		return "", fmt.Errorf("creating login page request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", browserUA)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching login page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading login page: %w", err)
	}

	matches := csrfPattern.FindSubmatch(body)
	if matches == nil {
		return "", fmt.Errorf("CSRF token not found on login page")
	}
	return string(matches[1]), nil
}

func submitLogin(
	ctx context.Context,
	httpClient *http.Client,
	baseURL, email, password, csrf string,
) (string, error) {
	form := url.Values{
		"authenticity_token": {csrf},
		"user[email]":        {email},
		"user[password]":     {password},
		"user[remember_me]":  {"1"},
		"commit":             {"Log in"},
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		baseURL+"/users/sign_in",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Referer", baseURL+"/login")
	req.Header.Set("User-Agent", browserUA)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("submitting login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("login failed: invalid email or password")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing base URL: %w", err)
	}
	for _, c := range httpClient.Jar.Cookies(parsed) {
		if c.Name == "frontend_api_token" {
			return c.Value, nil
		}
	}

	return "", fmt.Errorf(
		"frontend_api_token cookie not found after login (HTTP %d)",
		resp.StatusCode,
	)
}

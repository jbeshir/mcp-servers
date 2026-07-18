// Package httpx provides the shared HTTP client used by remote asset providers: a *http.Client whose
// transport stamps a default User-Agent on requests that don't already set one, plus small JSON/bytes
// helpers that map a non-2xx response to a typed error.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultUserAgent is stamped on outgoing requests that do not already set a User-Agent header.
const defaultUserAgent = "assets-mcp/0.1 (+https://github.com/jbeshir/mcp-servers)"

// defaultTimeout is the client timeout used when Config.Timeout is zero.
const defaultTimeout = 30 * time.Second

// Config configures the shared HTTP client. Zero values select sane defaults.
type Config struct {
	UserAgent string        // default: defaultUserAgent
	Timeout   time.Duration // default: 30s
}

// Client is the shared HTTP client for remote providers: a *http.Client whose transport stamps a
// default User-Agent on requests that do not already set one, plus small JSON/bytes helpers that map a
// non-2xx status to a typed error.
type Client struct {
	httpClient *http.Client
}

// userAgentTransport wraps a base http.RoundTripper, stamping a default User-Agent on requests that
// leave it unset. A caller-set User-Agent is never overridden.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

// RoundTrip stamps the default User-Agent on req when it has none set, then delegates to the base
// transport. The request is cloned before mutation, per http.RoundTripper's contract.
func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(req)
}

// StatusError reports that a request completed with a non-2xx HTTP status.
type StatusError struct {
	StatusCode int
	URL        string
}

// Error returns a human-readable description of the failed request.
func (e *StatusError) Error() string {
	return fmt.Sprintf("httpx: %s: unexpected status %d", e.URL, e.StatusCode)
}

// IsStatus reports whether err is, or wraps, a *StatusError carrying the given HTTP status code.
func IsStatus(err error, code int) bool {
	var se *StatusError
	return errors.As(err, &se) && se.StatusCode == code
}

// CheckStatus reports resp's status as a *StatusError for url when it is not 2xx, or nil otherwise.
func CheckStatus(resp *http.Response, url string) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{StatusCode: resp.StatusCode, URL: url}
	}
	return nil
}

// New builds a Client from cfg, applying defaults for zero fields.
func New(cfg Config) *Client {
	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: userAgentTransport{base: http.DefaultTransport, userAgent: userAgent},
		},
	}
}

// GetJSON issues a GET to url and decodes the JSON response body into v. A non-2xx status is reported
// as a *StatusError.
func (c *Client) GetJSON(ctx context.Context, url string, v any) error {
	return c.getJSON(ctx, url, nil, v)
}

// GetJSONHeaders issues a GET to url, setting each non-empty value in header on the request (e.g.
// Authorization), and decodes the JSON response body into v. A non-2xx status is reported as a
// *StatusError.
func (c *Client) GetJSONHeaders(ctx context.Context, url string, header http.Header, v any) error {
	return c.getJSON(ctx, url, header, v)
}

// getJSON is the shared implementation behind GetJSON and GetJSONHeaders.
func (c *Client) getJSON(ctx context.Context, url string, header http.Header, v any) error {
	resp, err := c.getOK(ctx, url, header)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("httpx: decode %s: %w", url, err)
	}
	return nil
}

// GetBytes issues a GET to url and returns the raw response body. A non-2xx status is reported as a
// *StatusError.
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	return c.getBytes(ctx, url, nil)
}

// GetBytesHeaders issues a GET to url, setting each non-empty value in header on the request (e.g.
// Authorization), and returns the raw response body. A non-2xx status is reported as a *StatusError.
func (c *Client) GetBytesHeaders(ctx context.Context, url string, header http.Header) ([]byte, error) {
	return c.getBytes(ctx, url, header)
}

// getBytes is the shared implementation behind GetBytes and GetBytesHeaders.
func (c *Client) getBytes(ctx context.Context, url string, header http.Header) ([]byte, error) {
	resp, err := c.getOK(ctx, url, header)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("httpx: read %s: %w", url, err)
	}
	return data, nil
}

// getOK issues a GET to url, setting each non-empty value in header on the request (header may be
// nil), and returns the response once its status has been confirmed 2xx. The caller is responsible for
// reading and closing the response body. A non-2xx status closes the body itself and returns a
// *StatusError.
func (c *Client) getOK(ctx context.Context, url string, header http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("httpx: build request for %s: %w", url, err)
	}
	for key, values := range header {
		for _, value := range values {
			if value != "" {
				req.Header.Add(key, value)
			}
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpx: get %s: %w", url, err)
	}

	if err := CheckStatus(resp, url); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

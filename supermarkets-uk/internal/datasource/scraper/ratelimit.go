package scraper

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimitedTransport wraps an http.RoundTripper and waits on a rate limiter
// before each request.
type RateLimitedTransport struct {
	Base    http.RoundTripper
	Limiter *rate.Limiter
}

// RoundTrip waits for the rate limiter then delegates to the base transport.
func (t *RateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.Limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.Base.RoundTrip(req)
}

// NewRateLimitedClient returns an *http.Client that rate-limits all requests.
func NewRateLimitedClient(limiter *rate.Limiter) *http.Client {
	return &http.Client{
		Transport: &RateLimitedTransport{
			Base:    http.DefaultTransport,
			Limiter: limiter,
		},
	}
}

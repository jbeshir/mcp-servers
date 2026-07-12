// Package ratelimit provides a thin politeness wrapper over golang.org/x/time/rate, used by remote
// asset providers to cap their outbound request rate to a single upstream host.
package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter is a thin politeness wrapper over golang.org/x/time/rate for one remote host.
type Limiter struct {
	lim *rate.Limiter
}

// New returns a Limiter allowing rps requests per second, with a burst of at least 1 (a burst below 1
// is raised to 1).
func New(rps float64, burst int) *Limiter {
	if burst < 1 {
		burst = 1
	}
	return &Limiter{lim: rate.NewLimiter(rate.Limit(rps), burst)}
}

// Wait blocks until a request may proceed, or returns ctx's error if it is cancelled or expires first.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.lim.Wait(ctx)
}

// Package wrapper provides datasource wrappers that add cross-cutting behaviour.
package wrapper

import (
	"context"

	"golang.org/x/time/rate"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// RateLimited wraps a Datasource and rate-limits every network-facing method.
type RateLimited struct {
	inner   datasource.Datasource
	limiter *rate.Limiter
}

// NewRateLimited returns a Datasource that waits on limiter before each call.
func NewRateLimited(ds datasource.Datasource, limiter *rate.Limiter) *RateLimited {
	return &RateLimited{inner: ds, limiter: limiter}
}

// ID returns the supermarket identifier.
func (d *RateLimited) ID() datasource.SupermarketID { return d.inner.ID() }

// Name returns the human-readable name.
func (d *RateLimited) Name() string { return d.inner.Name() }

// Description returns a short description of the supermarket.
func (d *RateLimited) Description() string { return d.inner.Description() }

// SearchProducts waits for the rate limiter then delegates.
func (d *RateLimited) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	if err := d.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return d.inner.SearchProducts(ctx, query)
}

// GetProductDetails waits for the rate limiter then delegates.
func (d *RateLimited) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	if err := d.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return d.inner.GetProductDetails(ctx, productID)
}

// BrowseCategories waits for the rate limiter then delegates.
func (d *RateLimited) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	if err := d.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return d.inner.BrowseCategories(ctx)
}

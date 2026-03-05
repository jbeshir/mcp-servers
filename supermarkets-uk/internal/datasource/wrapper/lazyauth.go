package wrapper

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// LoginFunc performs interactive login and returns session cookies.
type LoginFunc func(ctx context.Context) ([]*http.Cookie, error)

// LazyAuth wraps a datasource and defers interactive login until the
// supermarket is first queried. On successful login, cookies are
// injected into the inner datasource via SetCookies.
type LazyAuth struct {
	mu       sync.Mutex
	inner    datasource.AuthDatasource
	resolved bool
	doLogin  LoginFunc
}

// NewLazyAuth creates a lazy-auth wrapper.
func NewLazyAuth(
	inner datasource.AuthDatasource,
	doLogin LoginFunc,
) *LazyAuth {
	return &LazyAuth{
		inner:   inner,
		doLogin: doLogin,
	}
}

func (d *LazyAuth) resolve(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.resolved {
		return
	}
	d.resolved = true

	id := d.inner.ID()
	log.Printf(
		"first use of %s — starting interactive login "+
			"(complete login in the browser window)...", id,
	)

	cookies, err := d.doLogin(ctx)
	if err != nil {
		log.Printf(
			"warning: login failed for %s: %v "+
				"(using unauthenticated mode)", id, err,
		)
		return
	}

	d.inner.SetCookies(cookies)
	log.Printf("login successful for %s", id)
}

// ID returns the supermarket identifier.
func (d *LazyAuth) ID() datasource.SupermarketID {
	return d.inner.ID()
}

// Name returns the human-readable name.
func (d *LazyAuth) Name() string {
	return d.inner.Name()
}

// Description returns a short description of the supermarket.
func (d *LazyAuth) Description() string {
	return d.inner.Description()
}

// SearchProducts triggers login on first call, then delegates.
func (d *LazyAuth) SearchProducts(
	ctx context.Context, query string,
) ([]datasource.Product, error) {
	d.resolve(ctx)
	return d.inner.SearchProducts(ctx, query)
}

// GetProductDetails triggers login on first call, then delegates.
func (d *LazyAuth) GetProductDetails(
	ctx context.Context, productID string,
) (*datasource.Product, error) {
	d.resolve(ctx)
	return d.inner.GetProductDetails(ctx, productID)
}

// BrowseCategories triggers login on first call, then delegates.
func (d *LazyAuth) BrowseCategories(
	ctx context.Context,
) ([]datasource.Category, error) {
	d.resolve(ctx)
	return d.inner.BrowseCategories(ctx)
}

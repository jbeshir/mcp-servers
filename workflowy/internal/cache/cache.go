package cache

import (
	"context"
	"sync"
	"time"

	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
)

// Fetcher is a function that fetches all nodes from the API.
type Fetcher func(ctx context.Context) ([]client.Node, error)

// Cache provides TTL-based caching of the full Workflowy node export.
type Cache struct {
	mu        sync.RWMutex
	nodes     []client.Node
	fetchedAt time.Time
	ttl       time.Duration
	fetcher   Fetcher
}

// NewCache creates a new export cache.
// The minimum TTL is 60s to respect Workflowy's export rate limit.
func NewCache(fetcher Fetcher, ttl time.Duration) *Cache {
	if ttl < 60*time.Second {
		ttl = 60 * time.Second
	}
	return &Cache{
		fetcher: fetcher,
		ttl:     ttl,
	}
}

// GetAllNodes returns the cached nodes, re-fetching if the cache is stale.
// Uses double-checked locking to coalesce concurrent fetches.
func (c *Cache) GetAllNodes(ctx context.Context) ([]client.Node, error) {
	c.mu.RLock()
	if c.nodes != nil && time.Since(c.fetchedAt) < c.ttl {
		nodes := c.nodes
		c.mu.RUnlock()
		return nodes, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if c.nodes != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.nodes, nil
	}

	nodes, err := c.fetcher(ctx)
	if err != nil {
		return nil, err
	}

	c.nodes = nodes
	c.fetchedAt = time.Now()
	return nodes, nil
}

// Invalidate clears the cache so the next GetAllNodes call re-fetches.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes = nil
	c.fetchedAt = time.Time{}
}

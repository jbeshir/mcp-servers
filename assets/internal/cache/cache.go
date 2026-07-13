// Package cache provides a small on-disk byte cache keyed by an opaque fetch-identity string, used by
// remote asset providers to avoid re-fetching bytes they've already downloaded.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// dirPerm is the permission mode used when creating the cache directory.
const dirPerm = 0o750

// Cache is a small on-disk byte cache keyed by an opaque fetch-identity string, hashed with sha256 to a
// file under a base directory.
type Cache struct {
	dir string
}

// New returns a Cache storing entries under dir. The directory is created lazily on first Put.
func New(dir string) *Cache {
	return &Cache{dir: dir}
}

// Get returns the cached bytes for key. A cache miss (no file for key) returns (nil, false, nil), not
// an error; an error is returned only on a real IO fault.
func (c *Cache) Get(key string) ([]byte, bool, error) {
	data, err := os.ReadFile(c.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache: read %s: %w", key, err)
	}
	return data, true, nil
}

// Put stores data under key, creating the cache directory if needed and writing atomically via a
// temporary file followed by a rename.
func (c *Cache) Put(key string, data []byte) error {
	if err := os.MkdirAll(c.dir, dirPerm); err != nil {
		return fmt.Errorf("cache: create dir %s: %w", c.dir, err)
	}

	tmp, err := os.CreateTemp(c.dir, "tmp-*")
	if err != nil {
		return fmt.Errorf("cache: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: write %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, c.path(key)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: rename into place for %s: %w", key, err)
	}
	return nil
}

// Key joins provider and parts with a "\x00" separator, a byte that cannot appear in any of them,
// guaranteeing distinct providers or part sequences never collide on the same cache key.
func Key(provider string, parts ...string) string {
	return provider + "\x00" + strings.Join(parts, "\x00")
}

// path returns the on-disk path for key, hashed with sha256 to a hex filename.
func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

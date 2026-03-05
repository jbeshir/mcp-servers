package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// SerializedCookie is a JSON-friendly representation of an http.Cookie.
type SerializedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HttpOnly bool      `json:"httpOnly"`
	SameSite int       `json:"sameSite"`
}

// CookieStore saves and loads cookies as JSON files on disk.
type CookieStore struct {
	dir string
}

// NewCookieStore creates a new CookieStore that persists cookies in the given directory.
func NewCookieStore(dir string) (*CookieStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cookie dir: %w", err)
	}
	return &CookieStore{dir: dir}, nil
}

// DefaultCookieDir returns the cookie storage path.
// If SUPERMARKET_COOKIE_DIR is set, that path is used directly.
// Otherwise falls back to os.UserConfigDir.
func DefaultCookieDir() (string, error) {
	if dir := os.Getenv("SUPERMARKET_COOKIE_DIR"); dir != "" {
		return dir, nil
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get config dir: %w", err)
	}
	return filepath.Join(cfgDir, "supermarkets-uk-mcp", "cookies"), nil
}

func (s *CookieStore) path(id datasource.SupermarketID) string {
	return filepath.Join(s.dir, string(id)+".json")
}

// Load reads cookies for the given supermarket from disk.
// Returns nil, nil if no cookie file exists.
func (s *CookieStore) Load(id datasource.SupermarketID) ([]*http.Cookie, error) {
	data, err := os.ReadFile(s.path(id))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cookies for %s: %w", id, err)
	}

	var serialized []SerializedCookie
	if err := json.Unmarshal(data, &serialized); err != nil {
		return nil, fmt.Errorf("decode cookies for %s: %w", id, err)
	}

	cookies := make([]*http.Cookie, len(serialized))
	for i, sc := range serialized {
		cookies[i] = &http.Cookie{
			Name:     sc.Name,
			Value:    sanitizeCookieValue(sc.Value),
			Domain:   sc.Domain,
			Path:     sc.Path,
			Expires:  sc.Expires,
			Secure:   sc.Secure,
			HttpOnly: sc.HttpOnly,
			SameSite: http.SameSite(sc.SameSite),
		}
	}
	return cookies, nil
}

// Save writes cookies for the given supermarket to disk.
func (s *CookieStore) Save(id datasource.SupermarketID, cookies []*http.Cookie) error {
	serialized := make([]SerializedCookie, len(cookies))
	for i, c := range cookies {
		serialized[i] = SerializedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
			SameSite: int(c.SameSite),
		}
	}

	data, err := json.MarshalIndent(serialized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cookies for %s: %w", id, err)
	}

	if err := os.WriteFile(s.path(id), data, 0600); err != nil {
		return fmt.Errorf("write cookies for %s: %w", id, err)
	}
	return nil
}

// Clear removes stored cookies for the given supermarket.
func (s *CookieStore) Clear(id datasource.SupermarketID) error {
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

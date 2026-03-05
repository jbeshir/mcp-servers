package auth

import (
	"net/http"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func TestLoadCachedCookiesReturnsCached(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cached := []*http.Cookie{
		{Name: "OAuth.AccessToken", Value: "cached", Domain: ".tesco.com", Path: "/"},
	}
	if err := store.Save(datasource.Tesco, cached); err != nil {
		t.Fatal(err)
	}

	logins := map[datasource.SupermarketID]bool{datasource.Tesco: true}
	result := LoadCachedCookies(logins, store)

	cookies, ok := result[datasource.Tesco]
	if !ok || len(cookies) != 1 || cookies[0].Value != "cached" {
		t.Errorf("expected cached cookie, got %v", cookies)
	}
}

func TestLoadCachedCookiesSkipsUncached(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	logins := map[datasource.SupermarketID]bool{datasource.Tesco: true}
	result := LoadCachedCookies(logins, store)

	if _, ok := result[datasource.Tesco]; ok {
		t.Error("expected no cookies for Tesco (nothing cached)")
	}
}

func TestLoadCachedCookiesIgnoresUnflagged(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Save cookies for Tesco, but don't flag it.
	cached := []*http.Cookie{
		{Name: "OAuth.AccessToken", Value: "cached", Domain: ".tesco.com", Path: "/"},
	}
	if err := store.Save(datasource.Tesco, cached); err != nil {
		t.Fatal(err)
	}

	logins := map[datasource.SupermarketID]bool{} // No flags.
	result := LoadCachedCookies(logins, store)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

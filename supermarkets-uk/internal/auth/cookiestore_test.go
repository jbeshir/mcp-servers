package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// tescoDomain is the cookie domain used for Tesco test cookies.
const tescoDomain = ".tesco.com"

func TestCookieStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cookies := []*http.Cookie{
		{
			Name:     "session",
			Value:    "abc123",
			Domain:   tescoDomain,
			Path:     "/",
			Expires:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
		{
			Name:     "prefs",
			Value:    "dark",
			Domain:   tescoDomain,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
	}

	if err := store.Save(datasource.Tesco, cookies); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(datasource.Tesco)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(loaded))
	}

	if loaded[0].Name != "session" || loaded[0].Value != "abc123" {
		t.Errorf("cookie 0 = %+v", loaded[0])
	}
	assertLaxSecureFlags(t, loaded[0])
	if loaded[0].Domain != tescoDomain {
		t.Errorf("cookie 0 domain = %q", loaded[0].Domain)
	}

	if loaded[1].Name != "prefs" || loaded[1].Value != "dark" {
		t.Errorf("cookie 1 = %+v", loaded[1])
	}
}

func TestCookieStoreLoadNoFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cookies, err := store.Load(datasource.Tesco)
	if err != nil {
		t.Fatal(err)
	}
	if cookies != nil {
		t.Fatalf("expected nil cookies, got %v", cookies)
	}
}

func TestCookieStoreClear(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cookies := []*http.Cookie{{Name: "test", Value: "val", Domain: tescoDomain, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}}
	if err := store.Save(datasource.Tesco, cookies); err != nil {
		t.Fatal(err)
	}

	if err := store.Clear(datasource.Tesco); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(datasource.Tesco)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatalf("expected nil after clear, got %v", loaded)
	}
}

func TestCookieStoreClearNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Clear(datasource.Tesco); err != nil {
		t.Fatalf("clear nonexistent should not error, got %v", err)
	}
}

// assertLaxSecureFlags checks that Secure, HttpOnly and SameSite=Lax survived a store round trip.
func assertLaxSecureFlags(t *testing.T, c *http.Cookie) {
	t.Helper()
	if !c.Secure || !c.HttpOnly {
		t.Errorf("cookie flags: Secure=%v HttpOnly=%v", c.Secure, c.HttpOnly)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax", c.SameSite)
	}
}

// TestCookieStoreLoadAppliesSecureDefaults checks that Load forces Secure, HttpOnly and
// SameSite=Lax even when the stored cookie was saved without those flags.
func TestCookieStoreLoadAppliesSecureDefaults(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCookieStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal([]SerializedCookie{
		{Name: "theme", Value: "light", Domain: tescoDomain, Path: "/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path(datasource.Tesco), data, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(datasource.Tesco)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(loaded))
	}
	assertLaxSecureFlags(t, loaded[0])
}

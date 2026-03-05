package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

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
			Domain:   ".tesco.com",
			Path:     "/",
			Expires:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:   "prefs",
			Value:  "dark",
			Domain: ".tesco.com",
			Path:   "/",
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
	if !loaded[0].Secure || !loaded[0].HttpOnly {
		t.Errorf("cookie 0 flags: Secure=%v HttpOnly=%v", loaded[0].Secure, loaded[0].HttpOnly)
	}
	if loaded[0].Domain != ".tesco.com" {
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

	cookies := []*http.Cookie{{Name: "test", Value: "val", Domain: ".tesco.com", Path: "/"}}
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

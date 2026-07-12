package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientStampsDefaultUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{})
	if _, err := c.GetBytes(t.Context(), srv.URL); err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if gotUA != defaultUserAgent {
		t.Fatalf("User-Agent = %q, want %q", gotUA, defaultUserAgent)
	}
}

func TestClientDoesNotOverrideCallerUserAgent(t *testing.T) {
	const customUA = "custom-agent/1.0"

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	req.Header.Set("User-Agent", customUA)

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	if gotUA != customUA {
		t.Fatalf("User-Agent = %q, want %q", gotUA, customUA)
	}
}

func TestGetBytesNon2xxReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{})
	_, err := c.GetBytes(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("errors.As(%v, *StatusError): false", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusNotFound)
	}
	if statusErr.URL != srv.URL {
		t.Fatalf("URL = %q, want %q", statusErr.URL, srv.URL)
	}
}

func TestGetJSONDecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"lucide","count":42}`))
	}))
	defer srv.Close()

	c := New(Config{})
	var got struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := c.GetJSON(t.Context(), srv.URL, &got); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if got.Name != "lucide" || got.Count != 42 {
		t.Fatalf("got %+v, want Name=lucide Count=42", got)
	}
}

func TestGetBytesReturnsRawBody(t *testing.T) {
	const want = "raw svg bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(want))
	}))
	defer srv.Close()

	c := New(Config{})
	got, err := c.GetBytes(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(got) != want {
		t.Fatalf("GetBytes = %q, want %q", got, want)
	}
}

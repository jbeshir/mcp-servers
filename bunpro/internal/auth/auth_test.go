package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const loginPath = "/login"

// expectedCSRF is the authenticity_token value embedded in testdata/login_page.html.
const expectedCSRF = "aC3VIrO3klZz7IJ3D8hqPW9GKnC9NH3R-" +
	"6dyiKKs1Us8MTaJLAqWTXneeQo5qpcPR_QRIwQjyuROULodqmWz-w"

func readLoginPage(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/login_page.html")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// loginHandler serves the login page on GET and validates the form POST.
// It verifies the CSRF token from the fixture is forwarded correctly.
func loginHandler(t *testing.T, loginPage []byte) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET " + loginPath:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(loginPage)

		case "POST /users/sign_in":
			handleSignIn(t, w, r)

		default:
			http.NotFound(w, r)
		}
	})
}

func handleSignIn(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	// Verify the CSRF token extracted from the login page is forwarded.
	if got := r.FormValue("authenticity_token"); got != expectedCSRF {
		t.Errorf("authenticity_token = %q, want %q", got, expectedCSRF)
		http.Error(w, "wrong CSRF token", http.StatusUnprocessableEntity)
		return
	}
	if r.FormValue("user[email]") != "test@example.com" ||
		r.FormValue("user[password]") != "correct-password" {
		http.Error(w, "Invalid Email or password.", http.StatusUnauthorized)
		return
	}
	if r.FormValue("user[remember_me]") != "1" {
		t.Error("expected user[remember_me]=1")
	}
	if r.FormValue("commit") != "Log in" {
		t.Errorf("commit = %q, want 'Log in'", r.FormValue("commit"))
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "frontend_api_token",
		Value: "test-token-abc123",
		Path:  "/",
	})
	w.Header().Set("Location", "/dashboard")
	w.WriteHeader(http.StatusFound)
}

func TestLogin(t *testing.T) {
	loginPage := readLoginPage(t)
	srv := httptest.NewServer(loginHandler(t, loginPage))
	t.Cleanup(srv.Close)

	token, err := Login(context.Background(), srv.URL, "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "test-token-abc123" {
		t.Errorf("token = %q, want test-token-abc123", token)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	loginPage := readLoginPage(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET " + loginPath:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(loginPage)
		case "POST /users/sign_in":
			http.Error(w, "Invalid Email or password.", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	_, err := Login(context.Background(), srv.URL, "test@example.com", "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "invalid email or password") {
		t.Errorf("error should mention invalid credentials: %v", err)
	}
}

func TestLoginMissingCSRF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method+" "+r.URL.Path == "GET "+loginPath {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>no form</body></html>"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	_, err := Login(context.Background(), srv.URL, "test@example.com", "password")
	if err == nil {
		t.Fatal("expected error for missing CSRF token")
	}
	if !strings.Contains(err.Error(), "CSRF") {
		t.Errorf("error should mention CSRF: %v", err)
	}
}

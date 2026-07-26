package discord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGuildEmojiRequestsAndUpload(t *testing.T) {
	var requests []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /guilds/100/emojis", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"200","name":"wave","animated":false,"available":true}]`)
	})
	mux.HandleFunc("GET /guilds/100/emojis/200", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"id":"200","name":"wave","animated":false,"available":true}`)
	})
	mux.HandleFunc("POST /guilds/100/emojis", func(w http.ResponseWriter, r *http.Request) {
		assertUploadRequest(t, r)
		_, _ = io.WriteString(w, `{"id":"300","name":"new_emoji","available":true}`)
	})
	mux.HandleFunc("DELETE /guilds/100/emojis/300", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bot token" {
			t.Error("missing bot authorization")
		}
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	client := NewClient("token", server.Client())
	client.baseURL = server.URL
	if emojis, err := client.ListGuildEmojis(context.Background(), "100"); err != nil ||
		len(emojis) != 1 || emojis[0].ID != "200" {
		t.Fatalf("list: %#v %v", emojis, err)
	}
	if emoji, err := client.GetGuildEmoji(context.Background(), "100", "200"); err != nil ||
		emoji.Name != "wave" {
		t.Fatalf("get: %#v %v", emoji, err)
	}
	if _, err := client.CreateGuildEmoji(
		context.Background(), "100", "new_emoji", []byte("png"), []string{"123"}, "reason",
	); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteGuildEmoji(context.Background(), "100", "300", "reason"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 4 {
		t.Fatalf("unexpected requests: %v", requests)
	}
}

func assertUploadRequest(t *testing.T, r *http.Request) {
	t.Helper()
	var body struct {
		Name  string   `json:"name"`
		Image string   `json:"image"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	wantImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png"))
	if body.Name != "new_emoji" || body.Image != wantImage ||
		len(body.Roles) != 1 || body.Roles[0] != "123" {
		t.Errorf("unexpected upload body: %#v", body)
	}
}

func TestFetchEmojiImageOffline(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      int
		contentType string
		wantError   bool
	}{
		{"success", http.StatusOK, "image/webp; charset=binary", false},
		{"bad status", http.StatusNotFound, "image/webp", true},
		{"bad content type", http.StatusOK, "text/html", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "cdn.discordapp.com" ||
					!strings.Contains(r.URL.String(), "/emojis/200.webp?size=128") {
					t.Fatalf("unexpected CDN URL: %s", r.URL)
				}
				return &http.Response{
					StatusCode: tt.status,
					Header:     http.Header{"Content-Type": []string{tt.contentType}},
					Body:       io.NopCloser(strings.NewReader("image")),
				}, nil
			})
			client := NewClient("token", &http.Client{Transport: transport})
			data, contentType, err := client.FetchEmojiImage(
				context.Background(), Emoji{ID: "200", Name: "wave"},
			)
			if (err != nil) != tt.wantError {
				t.Fatalf("error=%v", err)
			}
			if !tt.wantError && (string(data) != "image" || contentType != "image/webp") {
				t.Fatalf("unexpected response: %q %q", data, contentType)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

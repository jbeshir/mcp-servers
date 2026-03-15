package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/bunpro/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newToolsTestServer(t *testing.T, routes map[string]string) *Server {
	t.Helper()
	fixtures := make(map[string][]byte, len(routes))
	for path, body := range routes {
		fixtures["/api/frontend"+path] = []byte(body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip query params for matching
		path := r.URL.Path
		data, ok := fixtures[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return NewServer(client.NewClient(srv.URL, ""))
}

func callTool(t *testing.T, s *Server, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	// Route to the right handler based on tool name
	var result *mcp.CallToolResult
	var err error
	ctx := context.Background()
	switch toolName {
	case "get_review_forecast":
		result, err = s.handleGetReviewForecast(ctx, req)
	case "get_grammar_srs_details":
		result, err = s.handleGetGrammarSRSDetails(ctx, req)
	case "get_vocab_srs_details":
		result, err = s.handleGetVocabSRSDetails(ctx, req)
	default:
		t.Fatalf("unknown tool: %s", toolName)
	}
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", toolName, err)
	}
	return result
}

func TestHandleGetReviewForecast_DefaultsToDaily(t *testing.T) {
	dailyJSON := `{"grammar":{"today":5,"tomorrow":10},"vocab":{"today":3}}`
	s := newToolsTestServer(t, map[string]string{
		"/user_stats/forecast_daily": dailyJSON,
	})

	result := callTool(t, s, "get_review_forecast", map[string]any{})
	text := resultText(t, result)

	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["grammar"]; !ok {
		t.Error("expected grammar key in daily forecast result")
	}
}

func TestHandleGetReviewForecast_HourlyWhenGranularitySet(t *testing.T) {
	hourlyJSON := `{"grammar":{"hour_0":2},"vocab":{"hour_1":1}}`
	s := newToolsTestServer(t, map[string]string{
		"/user_stats/forecast_hourly": hourlyJSON,
	})

	result := callTool(t, s, "get_review_forecast", map[string]any{"granularity": "hourly"})
	text := resultText(t, result)

	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["grammar"]; !ok {
		t.Error("expected grammar key in hourly forecast result")
	}
}

func TestHandleSRSDetails_MissingLevel(t *testing.T) {
	s := newToolsTestServer(t, map[string]string{})

	result := callTool(t, s, "get_grammar_srs_details", map[string]any{})
	if !result.IsError {
		t.Error("expected error result when level is missing")
	}
}

func TestHandleSRSDetails_InvalidLevel(t *testing.T) {
	s := newToolsTestServer(t, map[string]string{})

	result := callTool(t, s, "get_grammar_srs_details", map[string]any{"level": "grandmaster"})
	if !result.IsError {
		t.Error("expected error result for invalid level")
	}
	tc, _ := result.Content[0].(mcp.TextContent)
	if !strings.Contains(tc.Text, "beginner") {
		t.Errorf("error should list valid levels, got: %q", tc.Text)
	}
}

func TestHandleSRSDetails_PageDefaultsToOne(t *testing.T) {
	// Verify page=1 is used by default (URL will contain &page=1)
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reviews":{"data":[],"included":[]},"pagy":{"count":0,"page":1,"pages":0}}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	callTool(t, s, "get_grammar_srs_details", map[string]any{"level": "beginner"})

	if !strings.Contains(gotURL, "page=1") {
		t.Errorf("expected page=1 in URL, got: %q", gotURL)
	}
}

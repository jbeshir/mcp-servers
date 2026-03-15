package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/wanikani/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newToolsTestServer(t *testing.T, routes map[string]string) *Server {
	t.Helper()
	fixtures := make(map[string][]byte, len(routes))
	for path, body := range routes {
		fixtures[path] = []byte(body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := fixtures[r.URL.Path]
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

	ctx := context.Background()
	var result *mcp.CallToolResult
	var err error
	switch toolName {
	case "get_user":
		result, err = s.handleGetUser(ctx, req)
	case "get_summary":
		result, err = s.handleGetSummary(ctx, req)
	case "get_assignments":
		result, err = s.handleGetAssignments(ctx, req)
	case "get_subjects":
		result, err = s.handleGetSubjects(ctx, req)
	case "get_review_statistics":
		result, err = s.handleGetReviewStatistics(ctx, req)
	case "get_level_progressions":
		result, err = s.handleGetLevelProgressions(ctx, req)
	default:
		t.Fatalf("unknown tool: %s", toolName)
	}
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", toolName, err)
	}
	return result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want mcp.TextContent", result.Content[0])
	}
	return tc.Text
}

// Handler routing tests

func TestHandleGetUser_ReturnsUserJSON(t *testing.T) {
	userJSON := `{"id":1,"object":"user","data":{"username":"JBeshir","level":25}}`
	s := newToolsTestServer(t, map[string]string{
		"/v2/user": userJSON,
	})
	result := callTool(t, s, "get_user", nil)
	text := resultText(t, result)

	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["data"]; !ok {
		t.Error("expected data key in user result")
	}
}

func TestHandleGetAssignments_TranslatesParams(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"collection","total_count":0,"pages":{},"data":[]}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	callTool(t, s, "get_assignments", map[string]any{
		"subjectTypes": "kanji,radical",
		"levels":       "1,2",
	})

	if !strings.Contains(gotURL, "subject_types=kanji%2Cradical") && !strings.Contains(gotURL, "subject_types=kanji,radical") {
		t.Errorf("expected subject_types in URL, got: %q", gotURL)
	}
	if !strings.Contains(gotURL, "levels=1%2C2") && !strings.Contains(gotURL, "levels=1,2") {
		t.Errorf("expected levels in URL, got: %q", gotURL)
	}
}

func TestHandleGetSubjects_StripSpacesFromIDs(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"collection","total_count":0,"pages":{},"data":[]}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	callTool(t, s, "get_subjects", map[string]any{
		"ids": "1, 2, 3",
	})

	// Spaces should be stripped: "1, 2, 3" → "1,2,3"
	if strings.Contains(gotURL, " ") {
		t.Errorf("expected spaces stripped from ids, got URL: %q", gotURL)
	}
	if !strings.Contains(gotURL, "ids=1") {
		t.Errorf("expected ids in URL, got: %q", gotURL)
	}
}

// Param helper tests

func TestSetOptionalBool_SetsWhenTrue(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"collection","total_count":0,"pages":{},"data":[]}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	callTool(t, s, "get_assignments", map[string]any{
		"immediatelyAvailableForReview": true,
	})

	if !strings.Contains(gotURL, "immediately_available_for_review=true") {
		t.Errorf("expected bool param in URL, got: %q", gotURL)
	}
}

func TestSetOptionalBool_DropsOnWrongType(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"collection","total_count":0,"pages":{},"data":[]}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	// Pass a string where a bool is expected — should be silently dropped.
	callTool(t, s, "get_assignments", map[string]any{
		"immediatelyAvailableForReview": "yes",
	})

	if strings.Contains(gotURL, "immediately_available_for_review") {
		t.Errorf("wrong-type bool arg should be silently dropped, got URL: %q", gotURL)
	}
}

func TestSetOptionalNumber_DropsOnWrongType(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"collection","total_count":0,"pages":{},"data":[]}`))
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	// Pass a string where float64 is expected — should be silently dropped.
	callTool(t, s, "get_review_statistics", map[string]any{
		"percentagesGreaterThan": "80",
	})

	if strings.Contains(gotURL, "percentages_greater_than") {
		t.Errorf("wrong-type number arg should be silently dropped, got URL: %q", gotURL)
	}
}

func TestGetLimit_ReturnsZeroWhenAbsent(t *testing.T) {
	// getLimit returns 0 (sentinel for "use default") when limit arg is absent.
	limit := getLimit(map[string]any{})
	if limit != 0 {
		t.Errorf("absent limit should return 0, got %d", limit)
	}
}

func TestHandleGetUser_ReturnsErrorResultOnUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewServer(client.NewClient(srv.URL, ""))
	result := callTool(t, s, "get_user", nil)
	if !result.IsError {
		t.Error("expected IsError=true when upstream returns 404")
	}
}

func TestGetLimit_UsesProvidedValue(t *testing.T) {
	limit := getLimit(map[string]any{"limit": float64(42)})
	if limit != 42 {
		t.Errorf("limit = %d, want 42", limit)
	}
}

func TestGetLimit_DropsOnWrongType(t *testing.T) {
	limit := getLimit(map[string]any{"limit": "100"})
	if limit != 0 {
		t.Errorf("wrong-type limit should return 0, got %d", limit)
	}
}

// Format helper tests

func TestFormatCollection_NoItemsMessage(t *testing.T) {
	type Item struct{ Name string }
	result := formatCollection[Item]("widget", nil, 0)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "No widgets found.") {
		t.Errorf("unexpected empty-collection message: %q", text)
	}
}

func TestFormatCollection_Header(t *testing.T) {
	type Item struct{ Name string }
	items := []client.Resource[Item]{
		{ID: 1, Data: Item{Name: "first"}},
		{ID: 2, Data: Item{Name: "second"}},
	}
	result := formatCollection[Item]("widget", items, 100)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.HasPrefix(text, "Showing 2 of 100 widget(s):") {
		t.Errorf("unexpected header: %q", strings.SplitN(text, "\n", 2)[0])
	}
}

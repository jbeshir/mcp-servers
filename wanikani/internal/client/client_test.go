package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// newTestServer creates an httptest server that routes by request path
// to serve fixture JSON files from testdata/.
func newTestServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	fixtures := make(map[string][]byte, len(routes))
	for path, file := range routes {
		data, err := os.ReadFile(file) //nolint:gosec // test fixtures only
		if err != nil {
			t.Fatalf("reading fixture %s: %v", file, err)
		}
		fixtures[path] = data
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path
		data, ok := fixtures[reqPath]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Wanikani-Revision") != "20170710" {
			http.Error(w, "missing revision header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetUser(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/user": "testdata/user.json",
	})
	c := NewClient(srv.URL, "test-token")

	user, err := c.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Data.Username != "JBeshir" {
		t.Errorf("username = %q, want JBeshir", user.Data.Username)
	}
	if user.Data.Level != 4 {
		t.Errorf("level = %d, want 4", user.Data.Level)
	}
	if !user.Data.Subscription.Active {
		t.Error("subscription should be active")
	}
	if user.Data.Subscription.Type != "lifetime" {
		t.Errorf("subscription type = %q, want lifetime", user.Data.Subscription.Type)
	}
	if user.Data.Subscription.MaxLevelGranted != 60 {
		t.Errorf("max_level_granted = %d, want 60", user.Data.Subscription.MaxLevelGranted)
	}
}

func TestGetSummary(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/summary": "testdata/summary.json",
	})
	c := NewClient(srv.URL, "test-token")

	summary, err := c.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if len(summary.Data.Reviews) != 25 {
		t.Errorf("reviews entries = %d, want 25", len(summary.Data.Reviews))
	}
	if len(summary.Data.Lessons[0].SubjectIDs) != 15 {
		t.Errorf("lessons[0] subject_ids = %d, want 15", len(summary.Data.Lessons[0].SubjectIDs))
	}
}

func TestGetAssignments(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/assignments": "testdata/assignments.json",
	})
	c := NewClient(srv.URL, "test-token")

	items, total, err := c.GetAssignments(context.Background(), url.Values{"levels": {"1"}}, 0)
	if err != nil {
		t.Fatalf("GetAssignments: %v", err)
	}
	if total != 80 {
		t.Errorf("total = %d, want 80", total)
	}
	if len(items) != 80 {
		t.Errorf("items = %d, want 80", len(items))
	}
	first := items[0].Data
	if first.SubjectID != 6 {
		t.Errorf("first subject_id = %d, want 6", first.SubjectID)
	}
	if first.SubjectType != "radical" {
		t.Errorf("first subject_type = %q, want radical", first.SubjectType)
	}
	if first.SRSStage != 6 {
		t.Errorf("first srs_stage = %d, want 6", first.SRSStage)
	}
}

func TestGetSubjects(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/subjects": "testdata/subjects.json",
	})
	c := NewClient(srv.URL, "test-token")

	items, total, err := c.GetSubjects(
		context.Background(),
		url.Values{"types": {"radical"}, "levels": {"1"}}, 0,
	)
	if err != nil {
		t.Fatalf("GetSubjects: %v", err)
	}
	if total != 25 {
		t.Errorf("total = %d, want 25", total)
	}
	first := items[0].Data
	if first.Slug != "ground" {
		t.Errorf("first slug = %q, want ground", first.Slug)
	}
	if first.Level != 1 {
		t.Errorf("first level = %d, want 1", first.Level)
	}
	if first.Meanings[0].Meaning != "Ground" {
		t.Errorf("first meaning = %q, want Ground", first.Meanings[0].Meaning)
	}
}

func TestGetReviewStatistics(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/review_statistics": "testdata/review_statistics.json",
	})
	c := NewClient(srv.URL, "test-token")

	items, _, err := c.GetReviewStatistics(context.Background(), url.Values{}, 0)
	if err != nil {
		t.Fatalf("GetReviewStatistics: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items")
	}
	first := items[0].Data
	if first.SubjectID != 1 {
		t.Errorf("first subject_id = %d, want 1", first.SubjectID)
	}
	if first.PercentageCorrect != 100 {
		t.Errorf("first percentage_correct = %d, want 100", first.PercentageCorrect)
	}
}

func TestGetLevelProgressions(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/v2/level_progressions": "testdata/level_progressions.json",
	})
	c := NewClient(srv.URL, "test-token")

	items, _, err := c.GetLevelProgressions(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetLevelProgressions: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items")
	}
	if items[0].Data.Level != 1 {
		t.Errorf("first level = %d, want 1", items[0].Data.Level)
	}
}

func TestPagination(t *testing.T) {
	page1, err := os.ReadFile("testdata/assignments_page1.json")
	if err != nil {
		t.Fatal(err)
	}
	page2, err := os.ReadFile("testdata/assignments_page2.json")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "page_after_id") {
			_, _ = w.Write(page2)
		} else {
			body := strings.ReplaceAll(string(page1), "NEXT_URL_PLACEHOLDER",
				"http://"+r.Host+"/v2/assignments?levels=1&page_after_id=3")
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "test-token")
	items, total, err := c.GetAssignments(context.Background(), url.Values{}, 500)
	if err != nil {
		t.Fatalf("paginated GetAssignments: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want 5 (3 from page 1 + 2 from page 2)", len(items))
	}
}

func TestPaginationRespectsLimit(t *testing.T) {
	page1, err := os.ReadFile("testdata/assignments_page1.json")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := strings.ReplaceAll(string(page1), "NEXT_URL_PLACEHOLDER",
			"http://"+r.Host+"/v2/assignments?levels=1&page_after_id=3")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "test-token")
	items, _, err := c.GetAssignments(context.Background(), url.Values{}, 2)
	if err != nil {
		t.Fatalf("limited GetAssignments: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2 (limit)", len(items))
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "test-token")
	_, err := c.GetUser(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status 404: %v", err)
	}
}

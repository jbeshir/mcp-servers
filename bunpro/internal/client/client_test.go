package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// newTestServer creates an httptest server that routes by request path
// under /api/frontend/ to serve fixture JSON files from testdata/.
func newTestServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	fixtures := make(map[string][]byte, len(routes))
	for path, file := range routes {
		data, err := os.ReadFile(file) //nolint:gosec // test fixtures only
		if err != nil {
			t.Fatalf("reading fixture %s: %v", file, err)
		}
		fixtures["/api/frontend"+path] = data
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := fixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		"/user": "testdata/user.json",
	})
	c := NewClient(srv.URL, "test-token")

	user, err := c.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	attrs := user.User.Data.Attributes
	if attrs.Username != "JBeshir" {
		t.Errorf("username = %q, want JBeshir", attrs.Username)
	}
	if attrs.Level != 25 {
		t.Errorf("level = %d, want 25", attrs.Level)
	}
	if attrs.XP != 31220 {
		t.Errorf("xp = %d, want 31220", attrs.XP)
	}
	if !attrs.HasActiveSubscription {
		t.Error("expected active subscription")
	}
	if !attrs.IsLifetime {
		t.Error("expected lifetime subscription")
	}
}

func TestGetStudyQueue(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user/due": "testdata/due.json",
	})
	c := NewClient(srv.URL, "test-token")

	due, err := c.GetStudyQueue(context.Background())
	if err != nil {
		t.Fatalf("GetStudyQueue: %v", err)
	}
	if due.TotalDueGrammar != 1 {
		t.Errorf("total_due_grammar = %d, want 1", due.TotalDueGrammar)
	}
	if due.TotalDueVocab != 14 {
		t.Errorf("total_due_vocab = %d, want 14", due.TotalDueVocab)
	}
}

func TestGetDecks(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user/queue": "testdata/queue.json",
	})
	c := NewClient(srv.URL, "test-token")

	decks, err := c.GetDecks(context.Background())
	if err != nil {
		t.Fatalf("GetDecks: %v", err)
	}
	if len(decks.Data) != 2 {
		t.Fatalf("deck count = %d, want 2", len(decks.Data))
	}
	first := decks.Data[0].Attributes
	if first.DeckID != 1 {
		t.Errorf("first deck_id = %d, want 1", first.DeckID)
	}
	if first.DailyGoal != 1 {
		t.Errorf("first daily_goal = %d, want 1", first.DailyGoal)
	}
}

func TestGetBaseStats(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/base_stats": "testdata/base_stats.json",
	})
	c := NewClient(srv.URL, "test-token")

	stats, err := c.GetBaseStats(context.Background())
	if err != nil {
		t.Fatalf("GetBaseStats: %v", err)
	}
	if stats.Facts.Streak != 41 {
		t.Errorf("streak = %d, want 41", stats.Facts.Streak)
	}
	if stats.Facts.DaysStudied != 41 {
		t.Errorf("days_studied = %d, want 41", stats.Facts.DaysStudied)
	}
	if stats.Facts.GrammarStudied != 76 {
		t.Errorf("grammar_studied = %d, want 76", stats.Facts.GrammarStudied)
	}
	if len(stats.Badges.Data) != 3 {
		t.Errorf("badge count = %d, want 3", len(stats.Badges.Data))
	}
}

func TestGetJLPTProgress(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/jlpt_progress_mixed": "testdata/jlpt_progress.json",
	})
	c := NewClient(srv.URL, "test-token")

	progress, err := c.GetJLPTProgress(context.Background())
	if err != nil {
		t.Fatalf("GetJLPTProgress: %v", err)
	}
	if len(progress.Grammar) != 5 {
		t.Errorf("grammar JLPT levels = %d, want 5", len(progress.Grammar))
	}
	if len(progress.Vocab) != 5 {
		t.Errorf("vocab JLPT levels = %d, want 5", len(progress.Vocab))
	}
}

func TestGetForecastDaily(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/forecast_daily": "testdata/forecast_daily.json",
	})
	c := NewClient(srv.URL, "test-token")

	forecast, err := c.GetForecastDaily(context.Background())
	if err != nil {
		t.Fatalf("GetForecastDaily: %v", err)
	}
	if len(forecast.Grammar) == 0 {
		t.Error("expected grammar forecast entries")
	}
	if _, ok := forecast.Grammar["tomorrow"]; !ok {
		t.Error("expected 'tomorrow' key in grammar forecast")
	}
}

func TestGetForecastHourly(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/forecast_hourly": "testdata/forecast_hourly.json",
	})
	c := NewClient(srv.URL, "test-token")

	forecast, err := c.GetForecastHourly(context.Background())
	if err != nil {
		t.Fatalf("GetForecastHourly: %v", err)
	}
	if len(forecast.Vocab) == 0 {
		t.Error("expected vocab forecast entries")
	}
}

func TestGetSRSOverview(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/srs_level_overview": "testdata/srs_overview.json",
	})
	c := NewClient(srv.URL, "test-token")

	overview, err := c.GetSRSOverview(context.Background())
	if err != nil {
		t.Fatalf("GetSRSOverview: %v", err)
	}
	if overview.Grammar.Beginner != 5 {
		t.Errorf("grammar beginner = %d, want 5", overview.Grammar.Beginner)
	}
	if overview.Vocab.Adept != 196 {
		t.Errorf("vocab adept = %d, want 196", overview.Vocab.Adept)
	}
}

func TestGetReviewActivity(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/review_activity": "testdata/review_activity.json",
	})
	c := NewClient(srv.URL, "test-token")

	activity, err := c.GetReviewActivity(context.Background())
	if err != nil {
		t.Fatalf("GetReviewActivity: %v", err)
	}
	if len(activity.Grammar) == 0 {
		t.Error("expected grammar activity entries")
	}
	if len(activity.Vocab) == 0 {
		t.Error("expected vocab activity entries")
	}
}

func TestGetSRSLevelDetailsGrammar(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/srs_level_details": "testdata/srs_details.json",
	})
	c := NewClient(srv.URL, "test-token")

	details, err := c.GetSRSLevelDetails(context.Background(), SRSLevelBeginner, ReviewableTypeGrammarPoint, 1)
	if err != nil {
		t.Fatalf("GetSRSLevelDetails: %v", err)
	}
	if len(details.Reviews.Data) != 5 {
		t.Fatalf("review count = %d, want 5", len(details.Reviews.Data))
	}
	first := details.Reviews.Data[0].Attributes
	if first.ReviewableType != "GrammarPoint" {
		t.Errorf("reviewable_type = %q, want GrammarPoint", first.ReviewableType)
	}
	if first.ReviewableID != 48 {
		t.Errorf("reviewable_id = %d, want 48", first.ReviewableID)
	}

	// Verify included reviewable metadata is parsed.
	if len(details.Reviews.Included) != 5 {
		t.Fatalf("included count = %d, want 5", len(details.Reviews.Included))
	}
	firstInc := details.Reviews.Included[0].Attributes
	if firstInc.Title != "Verb + にいく" {
		t.Errorf("included[0] title = %q, want 'Verb + にいく'", firstInc.Title)
	}

	// Verify pagination metadata.
	if details.Pagy.Count != 5 {
		t.Errorf("pagy count = %d, want 5", details.Pagy.Count)
	}
	if details.Pagy.Pages != 1 {
		t.Errorf("pagy pages = %d, want 1", details.Pagy.Pages)
	}
}

func TestGetSRSLevelDetailsVocab(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/user_stats/srs_level_details": "testdata/srs_details_vocab.json",
	})
	c := NewClient(srv.URL, "test-token")

	details, err := c.GetSRSLevelDetails(context.Background(), SRSLevelBeginner, ReviewableTypeVocab, 1)
	if err != nil {
		t.Fatalf("GetSRSLevelDetails(Vocab): %v", err)
	}
	if len(details.Reviews.Data) != 40 {
		t.Fatalf("review count = %d, want 40", len(details.Reviews.Data))
	}
	first := details.Reviews.Data[0].Attributes
	if first.ReviewableType != "Vocab" {
		t.Errorf("reviewable_type = %q, want Vocab", first.ReviewableType)
	}

	// Verify included vocab metadata.
	if len(details.Reviews.Included) != 40 {
		t.Fatalf("included count = %d, want 40", len(details.Reviews.Included))
	}
	firstInc := details.Reviews.Included[0].Attributes
	if firstInc.Title != "電話" {
		t.Errorf("included[0] title = %q, want 電話", firstInc.Title)
	}
	if firstInc.Meaning != "telephone call, phone call" {
		t.Errorf("included[0] meaning = %q, want 'telephone call, phone call'",
			firstInc.Meaning)
	}

	// Verify pagination.
	if details.Pagy.Count != 40 {
		t.Errorf("pagy count = %d, want 40", details.Pagy.Count)
	}
}

const grammarDa = "だ"

func TestGetGrammarPoint(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/reviewables/grammar_point/1": "testdata/grammar_point.json",
	})
	c := NewClient(srv.URL, "test-token")

	gp, err := c.GetGrammarPoint(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetGrammarPoint: %v", err)
	}
	attrs := gp.Data.Attributes
	if attrs.Title != grammarDa {
		t.Errorf("title = %q, want %s", attrs.Title, grammarDa)
	}
	if attrs.Meaning != "To be, Is" {
		t.Errorf("meaning = %q, want 'To be, Is'", attrs.Meaning)
	}
	if attrs.Level != "JLPT5" {
		t.Errorf("level = %q, want JLPT5", attrs.Level)
	}
	if attrs.Slug != grammarDa {
		t.Errorf("slug = %q, want %s", attrs.Slug, grammarDa)
	}

	// Verify included study questions are parsed.
	var questions []StudyQuestion
	for _, inc := range gp.Included {
		if inc.Type == "study_question" && inc.Attributes.Answer != "" {
			questions = append(questions, inc.Attributes)
		}
	}
	if len(questions) == 0 {
		t.Fatal("expected study questions in included data")
	}
	if questions[0].Answer != grammarDa {
		t.Errorf("first study question answer = %q, want %s", questions[0].Answer, grammarDa)
	}
}

func TestGetVocab(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/reviewables/vocab/食べる": "testdata/vocab.json",
	})
	c := NewClient(srv.URL, "test-token")

	vocab, err := c.GetVocab(context.Background(), "食べる")
	if err != nil {
		t.Fatalf("GetVocab: %v", err)
	}
	attrs := vocab.Data.Attributes
	if attrs.Title != "食べる" {
		t.Errorf("title = %q, want 食べる", attrs.Title)
	}
	if attrs.JLPTLevel != "N5" {
		t.Errorf("jlpt_level = %q, want N5", attrs.JLPTLevel)
	}
	if attrs.Kana != "たべる" {
		t.Errorf("kana = %q, want たべる", attrs.Kana)
	}
	if attrs.PitchAccentStress != "LHL" {
		t.Errorf("pitch_accent = %q, want LHL", attrs.PitchAccentStress)
	}
	if attrs.JMDictData == nil {
		t.Fatal("expected JMDict data")
	}
	if len(attrs.JMDictData.Sense) == 0 {
		t.Fatal("expected at least one sense")
	}
	if attrs.JMDictData.Sense[0].Gloss[0].Text != "to eat" {
		t.Errorf("first gloss = %q, want 'to eat'",
			attrs.JMDictData.Sense[0].Gloss[0].Text)
	}
}

func TestGetVocabStudyQuestions(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/reviewables/vocab/食べる": "testdata/vocab.json",
	})
	c := NewClient(srv.URL, "test-token")

	vocab, err := c.GetVocab(context.Background(), "食べる")
	if err != nil {
		t.Fatalf("GetVocab: %v", err)
	}

	var questions []StudyQuestion
	for _, inc := range vocab.Included {
		if inc.Type == "study_question" {
			questions = append(questions, inc.Attributes)
		}
	}
	if len(questions) != 10 {
		t.Fatalf("study question count = %d, want 10", len(questions))
	}
	if questions[0].Answer != "たべる" {
		t.Errorf("first answer = %q, want たべる", questions[0].Answer)
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

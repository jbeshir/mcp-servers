package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/bunpro/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeDetailsResponse(reviews []client.Resource[client.Review], included []client.Resource[client.ReviewableBaseAttributes], page, pages, count int) *client.SRSLevelDetailsResponse {
	return &client.SRSLevelDetailsResponse{
		Reviews: client.ReviewsWithInc{
			Data:     reviews,
			Included: included,
		},
		Pagy: client.Pagy{Page: page, Pages: pages, Count: count},
	}
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

func TestFormatSRSDetails_SlugUsedAsLookupID(t *testing.T) {
	resp := makeDetailsResponse(
		[]client.Resource[client.Review]{
			{Attributes: client.Review{ReviewableID: 42, Streak: 3, Accuracy: 80}},
		},
		[]client.Resource[client.ReviewableBaseAttributes]{
			{Attributes: client.ReviewableBaseAttributes{ID: 42, Slug: "食べる", Title: "食べる", Meaning: "to eat"}},
		},
		1, 1, 1,
	)

	result, err := formatSRSDetails(resp)
	if err != nil {
		t.Fatalf("formatSRSDetails: %v", err)
	}

	text := resultText(t, result)
	body := strings.SplitN(text, "\n\n", 2)[1]

	var items []SRSDetailsSummaryItem
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].LookupID != "食べる" {
		t.Errorf("lookup_id = %q, want 食べる (slug)", items[0].LookupID)
	}
	if items[0].AccuracyPct != 80 {
		t.Errorf("accuracy_pct = %d, want 80", items[0].AccuracyPct)
	}
}

func TestFormatSRSDetails_NumericIDFallbackWhenNoSlug(t *testing.T) {
	resp := makeDetailsResponse(
		[]client.Resource[client.Review]{
			{Attributes: client.Review{ReviewableID: 99}},
		},
		[]client.Resource[client.ReviewableBaseAttributes]{
			{Attributes: client.ReviewableBaseAttributes{ID: 99, Slug: "", Title: "だ"}},
		},
		1, 1, 1,
	)

	result, err := formatSRSDetails(resp)
	if err != nil {
		t.Fatalf("formatSRSDetails: %v", err)
	}

	text := resultText(t, result)
	body := strings.SplitN(text, "\n\n", 2)[1]

	var items []SRSDetailsSummaryItem
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if items[0].LookupID != "99" {
		t.Errorf("lookup_id = %q, want 99 (numeric fallback)", items[0].LookupID)
	}
}

func TestFormatSRSDetails_PaginationHeader(t *testing.T) {
	resp := makeDetailsResponse(
		[]client.Resource[client.Review]{
			{Attributes: client.Review{ReviewableID: 1}},
		},
		[]client.Resource[client.ReviewableBaseAttributes]{
			{Attributes: client.ReviewableBaseAttributes{ID: 1}},
		},
		2, 5, 200,
	)

	result, err := formatSRSDetails(resp)
	if err != nil {
		t.Fatalf("formatSRSDetails: %v", err)
	}

	text := resultText(t, result)
	if !strings.HasPrefix(text, "Showing page 2 of 5 (200 items total):") {
		t.Errorf("unexpected header: %q", strings.SplitN(text, "\n", 2)[0])
	}
}

func TestFilterStudyQuestions_SkipsNonStudyQuestion(t *testing.T) {
	included := []client.Resource[client.StudyQuestion]{
		{Type: "study_question", Attributes: client.StudyQuestion{Content: "彼は学生__。", Answer: "だ", Translation: "He is a student."}},
		{Type: "writeup", Attributes: client.StudyQuestion{Content: "some writeup"}},
		{Type: "study_question", Attributes: client.StudyQuestion{Content: ""}}, // empty content skipped
	}

	questions := filterStudyQuestions(included)
	if len(questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(questions))
	}
	if questions[0].Answer != "だ" {
		t.Errorf("answer = %q, want だ", questions[0].Answer)
	}
}

package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jbeshir/mcp-servers/bunpro/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// SRSDetailsSummaryItem is the compact per-item representation
// returned to the MCP client.
type SRSDetailsSummaryItem struct {
	Title             string `json:"title"`
	Meaning           string `json:"meaning"`
	Level             string `json:"level"`
	Streak            int    `json:"streak"`
	AccuracyPct       int    `json:"accuracy_pct"`
	TimesStudied      int    `json:"times_studied"`
	StartedStudyingAt string `json:"started_studying_at"`
	NextReview        string `json:"next_review"`
	Ghosts            int    `json:"ghosts,omitempty"`
	LookupID          string `json:"lookup_id"`
}

func formatSRSDetails(
	resp *client.SRSLevelDetailsResponse,
) (*mcp.CallToolResult, error) {
	// Build a lookup from reviewable ID to included metadata.
	lookup := make(map[string]client.ReviewableBaseAttributes, len(resp.Reviews.Included))
	for _, inc := range resp.Reviews.Included {
		lookup[strconv.Itoa(inc.Attributes.ID)] = inc.Attributes
	}

	items := make([]SRSDetailsSummaryItem, 0, len(resp.Reviews.Data))
	for _, r := range resp.Reviews.Data {
		rev := r.Attributes
		idStr := strconv.Itoa(rev.ReviewableID)
		base := lookup[idStr]

		// Use slug for vocab (get_vocab accepts slugs), numeric ID for grammar.
		lookupID := base.Slug
		if lookupID == "" {
			lookupID = idStr
		}

		items = append(items, SRSDetailsSummaryItem{
			Title:             base.Title,
			Meaning:           base.Meaning,
			Level:             base.Level,
			Streak:            rev.Streak,
			AccuracyPct:       rev.Accuracy,
			TimesStudied:      rev.TimesStudied,
			StartedStudyingAt: rev.StartedStudyingAt,
			NextReview:        rev.NextReview,
			Ghosts:            rev.GhostCount,
			LookupID:          lookupID,
		})
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to format SRS details: %v", err)), nil
	}

	var header strings.Builder
	pagy := resp.Pagy
	fmt.Fprintf(&header, "Showing page %d of %d (%d items total):\n\n",
		pagy.Page, pagy.Pages, pagy.Count)

	return mcp.NewToolResultText(header.String() + string(data)), nil
}

// StudyQuestionSummary is a compact representation of a study question
// for grammar point and vocab detail responses.
type StudyQuestionSummary struct {
	Sentence    string `json:"sentence"`
	Answer      string `json:"answer"`
	Translation string `json:"translation"`
}

// filterStudyQuestions extracts study_question items from the included
// array and returns compact summaries. Non-study-question items
// (writeups, related_content) are skipped.
func filterStudyQuestions(
	included []client.Resource[client.StudyQuestion],
) []StudyQuestionSummary {
	var questions []StudyQuestionSummary
	for _, inc := range included {
		if inc.Type != "study_question" {
			continue
		}
		sq := inc.Attributes
		if sq.Content == "" {
			continue
		}
		questions = append(questions, StudyQuestionSummary{
			Sentence:    sq.Content,
			Answer:      sq.Answer,
			Translation: sq.Translation,
		})
	}
	return questions
}

// ReviewableDetail combines the item attributes with study questions.
type ReviewableDetail[T any] struct {
	Item      T                      `json:"item"`
	Questions []StudyQuestionSummary `json:"study_questions"`
}

func formatGrammarPoint(
	resp *client.GrammarPointResponse,
) (*mcp.CallToolResult, error) {
	detail := ReviewableDetail[client.GrammarPointAttributes]{
		Item:      resp.Data.Attributes,
		Questions: filterStudyQuestions(resp.Included),
	}
	return formatJSON(detail)
}

func formatVocab(
	resp *client.VocabResponse,
) (*mcp.CallToolResult, error) {
	detail := ReviewableDetail[client.VocabAttributes]{
		Item:      resp.Data.Attributes,
		Questions: filterStudyQuestions(resp.Included),
	}
	return formatJSON(detail)
}

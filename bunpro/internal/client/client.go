package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is an HTTP client for the Bunpro frontend API.
type Client struct {
	apiURL     string
	token      string
	httpClient *http.Client
}

// NewClient creates a new Bunpro API client.
func NewClient(apiURL, token string) *Client {
	return &Client{
		apiURL:     apiURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// do executes a GET request and decodes the JSON response into result.
func (c *Client) do(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL+"/api/frontend"+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// GetUser retrieves the authenticated user's profile.
func (c *Client) GetUser(ctx context.Context) (*UserResponse, error) {
	var r UserResponse
	if err := c.do(ctx, "/user", &r); err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &r, nil
}

// GetStudyQueue retrieves due review/lesson counts.
func (c *Client) GetStudyQueue(ctx context.Context) (*DueCount, error) {
	var r DueCount
	if err := c.do(ctx, "/user/due", &r); err != nil {
		return nil, fmt.Errorf("getting study queue: %w", err)
	}
	return &r, nil
}

// GetDecks retrieves the user's deck queue settings.
func (c *Client) GetDecks(ctx context.Context) (*CollectionEnvelope[DeckSetting], error) {
	var r CollectionEnvelope[DeckSetting]
	if err := c.do(ctx, "/user/queue", &r); err != nil {
		return nil, fmt.Errorf("getting decks: %w", err)
	}
	return &r, nil
}

// GetBaseStats retrieves the user's base statistics.
func (c *Client) GetBaseStats(ctx context.Context) (*BaseStats, error) {
	var r BaseStats
	if err := c.do(ctx, "/user_stats/base_stats", &r); err != nil {
		return nil, fmt.Errorf("getting base stats: %w", err)
	}
	return &r, nil
}

// GetJLPTProgress retrieves JLPT progress across all levels.
func (c *Client) GetJLPTProgress(ctx context.Context) (*JLPTProgress, error) {
	var r JLPTProgress
	if err := c.do(ctx, "/user_stats/jlpt_progress_mixed", &r); err != nil {
		return nil, fmt.Errorf("getting JLPT progress: %w", err)
	}
	return &r, nil
}

// GetForecastDaily retrieves the daily review forecast.
func (c *Client) GetForecastDaily(ctx context.Context) (*GrammarVocabMap, error) {
	var r GrammarVocabMap
	if err := c.do(ctx, "/user_stats/forecast_daily", &r); err != nil {
		return nil, fmt.Errorf("getting daily forecast: %w", err)
	}
	return &r, nil
}

// GetForecastHourly retrieves the hourly review forecast.
func (c *Client) GetForecastHourly(ctx context.Context) (*GrammarVocabMap, error) {
	var r GrammarVocabMap
	if err := c.do(ctx, "/user_stats/forecast_hourly", &r); err != nil {
		return nil, fmt.Errorf("getting hourly forecast: %w", err)
	}
	return &r, nil
}

// GetSRSOverview retrieves SRS level overview counts.
func (c *Client) GetSRSOverview(ctx context.Context) (*SRSOverview, error) {
	var r SRSOverview
	if err := c.do(ctx, "/user_stats/srs_level_overview", &r); err != nil {
		return nil, fmt.Errorf("getting SRS overview: %w", err)
	}
	return &r, nil
}

// GetReviewActivity retrieves review activity history.
func (c *Client) GetReviewActivity(ctx context.Context) (*GrammarVocabMap, error) {
	var r GrammarVocabMap
	if err := c.do(ctx, "/user_stats/review_activity", &r); err != nil {
		return nil, fmt.Errorf("getting review activity: %w", err)
	}
	return &r, nil
}

// GetSRSLevelDetails retrieves paginated reviews for a specific SRS level.
func (c *Client) GetSRSLevelDetails(
	ctx context.Context, level, reviewableType string, page int,
) (*SRSLevelDetailsResponse, error) {
	path := fmt.Sprintf("/user_stats/srs_level_details?level=%s&reviewable_type=%s", level, reviewableType)
	if page > 0 {
		path += fmt.Sprintf("&page=%d", page)
	}
	var r SRSLevelDetailsResponse
	if err := c.do(ctx, path, &r); err != nil {
		return nil, fmt.Errorf("getting SRS level details: %w", err)
	}
	return &r, nil
}

// GetGrammarPoint retrieves a grammar point by ID.
func (c *Client) GetGrammarPoint(ctx context.Context, id string) (*GrammarPointResponse, error) {
	var r GrammarPointResponse
	if err := c.do(ctx, "/reviewables/grammar_point/"+id, &r); err != nil {
		return nil, fmt.Errorf("getting grammar point %s: %w", id, err)
	}
	return &r, nil
}

// GetVocab retrieves a vocabulary item by slug or ID.
func (c *Client) GetVocab(ctx context.Context, slugOrID string) (*VocabResponse, error) {
	var r VocabResponse
	if err := c.do(ctx, "/reviewables/vocab/"+slugOrID, &r); err != nil {
		return nil, fmt.Errorf("getting vocab %s: %w", slugOrID, err)
	}
	return &r, nil
}

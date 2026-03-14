package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is an HTTP client for the WaniKani API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new WaniKani API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// do executes a GET request and decodes the JSON response into result.
func (c *Client) do(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Wanikani-Revision", "20170710")

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

// doCollection fetches a paginated collection, auto-following next_url up to limit items.
func doCollection[T any](c *Client, ctx context.Context, path string, limit int) ([]Resource[T], int, error) {
	if limit <= 0 {
		limit = 500
	}
	const maxLimit = 10000

	if limit > maxLimit {
		limit = maxLimit
	}

	var all []Resource[T]
	var totalCount int
	currentPath := path

	for currentPath != "" && len(all) < limit {
		var col Collection[T]
		if err := c.do(ctx, currentPath, &col); err != nil {
			return nil, 0, err
		}
		totalCount = col.TotalCount
		all = append(all, col.Data...)

		if col.Pages.NextURL == "" || len(all) >= limit {
			break
		}

		// next_url is absolute; strip the base URL to get a relative path.
		parsed, err := url.Parse(col.Pages.NextURL)
		if err != nil {
			return nil, 0, fmt.Errorf("parsing next_url: %w", err)
		}
		currentPath = parsed.RequestURI()
	}

	if len(all) > limit {
		all = all[:limit]
	}
	return all, totalCount, nil
}

// GetUser retrieves the authenticated user's profile.
func (c *Client) GetUser(ctx context.Context) (*Resource[User], error) {
	var r Resource[User]
	if err := c.do(ctx, "/v2/user", &r); err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &r, nil
}

// GetSummary retrieves the lesson/review summary.
func (c *Client) GetSummary(ctx context.Context) (*Resource[Summary], error) {
	var r Resource[Summary]
	if err := c.do(ctx, "/v2/summary", &r); err != nil {
		return nil, fmt.Errorf("getting summary: %w", err)
	}
	return &r, nil
}

// GetAssignments retrieves assignments with optional filters.
func (c *Client) GetAssignments(
	ctx context.Context, params url.Values, limit int,
) ([]Resource[Assignment], int, error) {
	path := "/v2/assignments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	items, total, err := doCollection[Assignment](c, ctx, path, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("getting assignments: %w", err)
	}
	return items, total, nil
}

// GetSubjects retrieves subjects with optional filters.
func (c *Client) GetSubjects(ctx context.Context, params url.Values, limit int) ([]Resource[Subject], int, error) {
	path := "/v2/subjects"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	items, total, err := doCollection[Subject](c, ctx, path, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("getting subjects: %w", err)
	}
	return items, total, nil
}

// GetReviewStatistics retrieves review statistics with optional filters.
func (c *Client) GetReviewStatistics(
	ctx context.Context, params url.Values, limit int,
) ([]Resource[ReviewStatistic], int, error) {
	path := "/v2/review_statistics"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	items, total, err := doCollection[ReviewStatistic](c, ctx, path, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("getting review statistics: %w", err)
	}
	return items, total, nil
}

// GetLevelProgressions retrieves level progressions.
func (c *Client) GetLevelProgressions(ctx context.Context, limit int) ([]Resource[LevelProgression], int, error) {
	items, total, err := doCollection[LevelProgression](c, ctx, "/v2/level_progressions", limit)
	if err != nil {
		return nil, 0, fmt.Errorf("getting level progressions: %w", err)
	}
	return items, total, nil
}

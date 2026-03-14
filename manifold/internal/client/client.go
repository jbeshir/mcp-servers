package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is an HTTP client for the Manifold Markets API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Manifold Markets API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// do executes an HTTP request and decodes the JSON response into result.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
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

// doJSON marshals payload as JSON and executes the request.
func (c *Client) doJSON(ctx context.Context, path string, payload, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, bytes.NewReader(data), result)
}

// SearchMarkets searches for markets using query parameters.
func (c *Client) SearchMarkets(ctx context.Context, params url.Values) ([]LiteMarket, error) {
	path := "/v0/search-markets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var markets []LiteMarket
	if err := c.do(ctx, http.MethodGet, path, nil, &markets); err != nil {
		return nil, fmt.Errorf("searching markets: %w", err)
	}
	return markets, nil
}

// GetMarket retrieves full details for a specific market.
func (c *Client) GetMarket(ctx context.Context, marketID string) (*FullMarket, error) {
	var market FullMarket
	if err := c.do(ctx, http.MethodGet, "/v0/market/"+marketID, nil, &market); err != nil {
		return nil, fmt.Errorf("getting market %s: %w", marketID, err)
	}
	return &market, nil
}

// GetUser retrieves a user profile by username.
func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodGet, "/v0/user/"+username, nil, &user); err != nil {
		return nil, fmt.Errorf("getting user %s: %w", username, err)
	}
	return &user, nil
}

// GetMe retrieves the authenticated user's profile.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodGet, "/v0/me", nil, &user); err != nil {
		return nil, fmt.Errorf("getting authenticated user: %w", err)
	}
	return &user, nil
}

// ListBets lists bets with the given query parameters.
func (c *Client) ListBets(ctx context.Context, params url.Values) ([]Bet, error) {
	path := "/v0/bets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var bets []Bet
	if err := c.do(ctx, http.MethodGet, path, nil, &bets); err != nil {
		return nil, fmt.Errorf("listing bets: %w", err)
	}
	return bets, nil
}

// GetComments retrieves comments with the given query parameters.
func (c *Client) GetComments(ctx context.Context, params url.Values) ([]Comment, error) {
	path := "/v0/comments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var comments []Comment
	if err := c.do(ctx, http.MethodGet, path, nil, &comments); err != nil {
		return nil, fmt.Errorf("getting comments: %w", err)
	}
	return comments, nil
}

// GetPositions retrieves user positions for a market.
func (c *Client) GetPositions(ctx context.Context, marketID string, params url.Values) ([]ContractMetric, error) {
	path := "/v0/market/" + marketID + "/positions"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var positions []ContractMetric
	if err := c.do(ctx, http.MethodGet, path, nil, &positions); err != nil {
		return nil, fmt.Errorf("getting positions for market %s: %w", marketID, err)
	}
	return positions, nil
}

// PlaceBet places a bet on a market.
func (c *Client) PlaceBet(ctx context.Context, req PlaceBetRequest) (*Bet, error) {
	var bet Bet
	if err := c.doJSON(ctx, "/v0/bet", req, &bet); err != nil {
		return nil, fmt.Errorf("placing bet: %w", err)
	}
	return &bet, nil
}

// SellShares sells shares in a market.
func (c *Client) SellShares(ctx context.Context, marketID string, req SellSharesRequest) (*Bet, error) {
	var bet Bet
	if err := c.doJSON(ctx, "/v0/market/"+marketID+"/sell", req, &bet); err != nil {
		return nil, fmt.Errorf("selling shares in market %s: %w", marketID, err)
	}
	return &bet, nil
}

// CancelBet cancels a limit order.
func (c *Client) CancelBet(ctx context.Context, betID string) error {
	if err := c.do(ctx, http.MethodPost, "/v0/bet/cancel/"+betID, nil, nil); err != nil {
		return fmt.Errorf("canceling bet %s: %w", betID, err)
	}
	return nil
}

// CreateMarket creates a new market.
func (c *Client) CreateMarket(ctx context.Context, req CreateMarketRequest) (*LiteMarket, error) {
	var market LiteMarket
	if err := c.doJSON(ctx, "/v0/market", req, &market); err != nil {
		return nil, fmt.Errorf("creating market: %w", err)
	}
	return &market, nil
}

// ResolveMarket resolves a market.
func (c *Client) ResolveMarket(ctx context.Context, marketID string, req ResolveMarketRequest) error {
	if err := c.doJSON(ctx, "/v0/market/"+marketID+"/resolve", req, nil); err != nil {
		return fmt.Errorf("resolving market %s: %w", marketID, err)
	}
	return nil
}

// CloseMarket closes a market.
func (c *Client) CloseMarket(ctx context.Context, marketID string, req CloseMarketRequest) error {
	if err := c.doJSON(ctx, "/v0/market/"+marketID+"/close", req, nil); err != nil {
		return fmt.Errorf("closing market %s: %w", marketID, err)
	}
	return nil
}

// AddComment adds a comment to a market.
func (c *Client) AddComment(ctx context.Context, req AddCommentRequest) (*Comment, error) {
	var comment Comment
	if err := c.doJSON(ctx, "/v0/comment", req, &comment); err != nil {
		return nil, fmt.Errorf("adding comment: %w", err)
	}
	return &comment, nil
}

// AddLiquidity adds liquidity to a market.
func (c *Client) AddLiquidity(ctx context.Context, marketID string, req AddLiquidityRequest) error {
	if err := c.doJSON(ctx, "/v0/market/"+marketID+"/add-liquidity", req, nil); err != nil {
		return fmt.Errorf("adding liquidity to market %s: %w", marketID, err)
	}
	return nil
}

// SendMana sends mana to users.
func (c *Client) SendMana(ctx context.Context, req SendManaRequest) error {
	if err := c.doJSON(ctx, "/v0/managram", req, nil); err != nil {
		return fmt.Errorf("sending mana: %w", err)
	}
	return nil
}

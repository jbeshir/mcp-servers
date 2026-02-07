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

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) doRequestJSON(ctx context.Context, path string, payload any) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	return c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(data))
}

func handleResponse(resp *http.Response, result any) error {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// SearchMarkets searches for markets using query parameters.
func (c *Client) SearchMarkets(ctx context.Context, params url.Values) ([]LiteMarket, error) {
	path := "/v0/search-markets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var markets []LiteMarket
	if err := handleResponse(resp, &markets); err != nil {
		return nil, fmt.Errorf("searching markets: %w", err)
	}
	return markets, nil
}

// GetMarket retrieves full details for a specific market.
func (c *Client) GetMarket(ctx context.Context, marketID string) (*FullMarket, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v0/market/"+marketID, nil)
	if err != nil {
		return nil, err
	}
	var market FullMarket
	if err := handleResponse(resp, &market); err != nil {
		return nil, fmt.Errorf("getting market %s: %w", marketID, err)
	}
	return &market, nil
}

// GetUser retrieves a user profile by username.
func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v0/user/"+username, nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := handleResponse(resp, &user); err != nil {
		return nil, fmt.Errorf("getting user %s: %w", username, err)
	}
	return &user, nil
}

// GetMe retrieves the authenticated user's profile.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v0/me", nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := handleResponse(resp, &user); err != nil {
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
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var bets []Bet
	if err := handleResponse(resp, &bets); err != nil {
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
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var comments []Comment
	if err := handleResponse(resp, &comments); err != nil {
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
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var positions []ContractMetric
	if err := handleResponse(resp, &positions); err != nil {
		return nil, fmt.Errorf("getting positions for market %s: %w", marketID, err)
	}
	return positions, nil
}

// PlaceBet places a bet on a market.
func (c *Client) PlaceBet(ctx context.Context, req PlaceBetRequest) (*Bet, error) {
	resp, err := c.doRequestJSON(ctx, "/v0/bet", req)
	if err != nil {
		return nil, err
	}
	var bet Bet
	if err := handleResponse(resp, &bet); err != nil {
		return nil, fmt.Errorf("placing bet: %w", err)
	}
	return &bet, nil
}

// SellShares sells shares in a market.
func (c *Client) SellShares(ctx context.Context, marketID string, req SellSharesRequest) (*Bet, error) {
	resp, err := c.doRequestJSON(ctx, "/v0/market/"+marketID+"/sell", req)
	if err != nil {
		return nil, err
	}
	var bet Bet
	if err := handleResponse(resp, &bet); err != nil {
		return nil, fmt.Errorf("selling shares in market %s: %w", marketID, err)
	}
	return &bet, nil
}

// CancelBet cancels a limit order.
func (c *Client) CancelBet(ctx context.Context, betID string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/v0/bet/cancel/"+betID, nil)
	if err != nil {
		return err
	}
	if err := handleResponse(resp, nil); err != nil {
		return fmt.Errorf("canceling bet %s: %w", betID, err)
	}
	return nil
}

// CreateMarket creates a new market.
func (c *Client) CreateMarket(ctx context.Context, req CreateMarketRequest) (*LiteMarket, error) {
	resp, err := c.doRequestJSON(ctx, "/v0/market", req)
	if err != nil {
		return nil, err
	}
	var market LiteMarket
	if err := handleResponse(resp, &market); err != nil {
		return nil, fmt.Errorf("creating market: %w", err)
	}
	return &market, nil
}

// ResolveMarket resolves a market.
func (c *Client) ResolveMarket(ctx context.Context, marketID string, req ResolveMarketRequest) error {
	resp, err := c.doRequestJSON(ctx, "/v0/market/"+marketID+"/resolve", req)
	if err != nil {
		return err
	}
	if err := handleResponse(resp, nil); err != nil {
		return fmt.Errorf("resolving market %s: %w", marketID, err)
	}
	return nil
}

// CloseMarket closes a market.
func (c *Client) CloseMarket(ctx context.Context, marketID string, req CloseMarketRequest) error {
	resp, err := c.doRequestJSON(ctx, "/v0/market/"+marketID+"/close", req)
	if err != nil {
		return err
	}
	if err := handleResponse(resp, nil); err != nil {
		return fmt.Errorf("closing market %s: %w", marketID, err)
	}
	return nil
}

// AddComment adds a comment to a market.
func (c *Client) AddComment(ctx context.Context, req AddCommentRequest) (*Comment, error) {
	resp, err := c.doRequestJSON(ctx, "/v0/comment", req)
	if err != nil {
		return nil, err
	}
	var comment Comment
	if err := handleResponse(resp, &comment); err != nil {
		return nil, fmt.Errorf("adding comment: %w", err)
	}
	return &comment, nil
}

// AddLiquidity adds liquidity to a market.
func (c *Client) AddLiquidity(ctx context.Context, marketID string, req AddLiquidityRequest) error {
	resp, err := c.doRequestJSON(ctx, "/v0/market/"+marketID+"/add-liquidity", req)
	if err != nil {
		return err
	}
	if err := handleResponse(resp, nil); err != nil {
		return fmt.Errorf("adding liquidity to market %s: %w", marketID, err)
	}
	return nil
}

// SendMana sends mana to users.
func (c *Client) SendMana(ctx context.Context, req SendManaRequest) error {
	resp, err := c.doRequestJSON(ctx, "/v0/managram", req)
	if err != nil {
		return err
	}
	if err := handleResponse(resp, nil); err != nil {
		return fmt.Errorf("sending mana: %w", err)
	}
	return nil
}

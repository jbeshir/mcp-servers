package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is an HTTP client for the Workflowy API.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a new Workflowy API client.
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiToken:   apiToken,
		httpClient: &http.Client{},
	}
}

// do executes an HTTP request and decodes the JSON response into result.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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
func (c *Client) doJSON(ctx context.Context, method, path string, payload, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}
	return c.do(ctx, method, path, bytes.NewReader(data), result)
}

// GetNode retrieves a single node by ID.
func (c *Client) GetNode(ctx context.Context, nodeID string) (*Node, error) {
	var wrapper nodeResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/nodes/"+nodeID, nil, &wrapper); err != nil {
		return nil, fmt.Errorf("getting node %s: %w", nodeID, err)
	}
	return &wrapper.Node, nil
}

// ListChildren lists children of a parent node.
// parentID can be a node UUID, a target key ("home", "inbox"), or "None" for top-level.
func (c *Client) ListChildren(ctx context.Context, parentID string) ([]Node, error) {
	path := "/api/v1/nodes"
	if parentID != "" {
		path += "?parent_id=" + parentID
	}
	var wrapper nodesResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &wrapper); err != nil {
		return nil, fmt.Errorf("listing children: %w", err)
	}
	return wrapper.Nodes, nil
}

// CreateNode creates a new node.
func (c *Client) CreateNode(ctx context.Context, req CreateNodeRequest) (*CreateNodeResponse, error) {
	var result CreateNodeResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes", req, &result); err != nil {
		return nil, fmt.Errorf("creating node: %w", err)
	}
	return &result, nil
}

// UpdateNode updates an existing node.
func (c *Client) UpdateNode(ctx context.Context, nodeID string, req UpdateNodeRequest) error {
	var result StatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes/"+nodeID, req, &result); err != nil {
		return fmt.Errorf("updating node %s: %w", nodeID, err)
	}
	return nil
}

// DeleteNode deletes a node by ID.
func (c *Client) DeleteNode(ctx context.Context, nodeID string) error {
	var result StatusResponse
	if err := c.do(ctx, http.MethodDelete, "/api/v1/nodes/"+nodeID, nil, &result); err != nil {
		return fmt.Errorf("deleting node %s: %w", nodeID, err)
	}
	return nil
}

// MoveNode moves a node to a new parent.
func (c *Client) MoveNode(ctx context.Context, nodeID string, req MoveNodeRequest) error {
	var result StatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes/"+nodeID+"/move", req, &result); err != nil {
		return fmt.Errorf("moving node %s: %w", nodeID, err)
	}
	return nil
}

// CompleteNode marks a node as completed.
func (c *Client) CompleteNode(ctx context.Context, nodeID string) error {
	var result StatusResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/nodes/"+nodeID+"/complete", nil, &result); err != nil {
		return fmt.Errorf("completing node %s: %w", nodeID, err)
	}
	return nil
}

// UncompleteNode marks a node as not completed.
func (c *Client) UncompleteNode(ctx context.Context, nodeID string) error {
	var result StatusResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/nodes/"+nodeID+"/uncomplete", nil, &result); err != nil {
		return fmt.Errorf("uncompleting node %s: %w", nodeID, err)
	}
	return nil
}

// ExportNodes exports all nodes as a flat list.
// Rate limited to 1 request per minute on the server side.
func (c *Client) ExportNodes(ctx context.Context) ([]Node, error) {
	var wrapper nodesResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/nodes-export", nil, &wrapper); err != nil {
		return nil, fmt.Errorf("exporting nodes: %w", err)
	}
	return wrapper.Nodes, nil
}

// ListTargets returns all targets (system locations and shortcuts).
func (c *Client) ListTargets(ctx context.Context) ([]Target, error) {
	var wrapper targetsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/targets", nil, &wrapper); err != nil {
		return nil, fmt.Errorf("listing targets: %w", err)
	}
	return wrapper.Targets, nil
}

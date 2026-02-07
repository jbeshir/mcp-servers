package client

// Node represents a Workflowy node/bullet.
type Node struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Note        *string  `json:"note"`
	ParentID    *string  `json:"parent_id,omitempty"`
	Priority    int      `json:"priority"`
	Data        NodeData `json:"data"`
	CreatedAt   int64    `json:"createdAt"`
	ModifiedAt  int64    `json:"modifiedAt"`
	CompletedAt *int64   `json:"completedAt"`
	Completed   *bool    `json:"completed,omitempty"`
}

// NodeData holds additional node properties.
type NodeData struct {
	LayoutMode string `json:"layoutMode,omitempty"`
}

// Target represents a Workflowy target (system location or shortcut).
type Target struct {
	Key  string  `json:"key"`
	Type string  `json:"type"`
	Name *string `json:"name"`
}

// CreateNodeRequest is the request body for creating a node.
type CreateNodeRequest struct {
	ParentID   string `json:"parent_id"`
	Name       string `json:"name"`
	Note       string `json:"note,omitempty"`
	LayoutMode string `json:"layoutMode,omitempty"`
	Position   string `json:"position,omitempty"`
}

// UpdateNodeRequest is the request body for updating a node.
type UpdateNodeRequest struct {
	Name       *string `json:"name,omitempty"`
	Note       *string `json:"note,omitempty"`
	LayoutMode *string `json:"layoutMode,omitempty"`
}

// MoveNodeRequest is the request body for moving a node.
type MoveNodeRequest struct {
	ParentID string `json:"parent_id,omitempty"`
	Position string `json:"position,omitempty"`
}

// API response wrappers.
type nodeResponse struct {
	Node Node `json:"node"`
}

type nodesResponse struct {
	Nodes []Node `json:"nodes"`
}

type targetsResponse struct {
	Targets []Target `json:"targets"`
}

// CreateNodeResponse is the response from creating a node.
type CreateNodeResponse struct {
	ItemID string `json:"item_id"`
}

// StatusResponse is the response from update/delete/move/complete operations.
type StatusResponse struct {
	Status string `json:"status"`
}

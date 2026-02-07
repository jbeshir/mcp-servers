package server

import (
	"github.com/jbeshir/mcp-servers/workflowy/internal/cache"
	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for Workflowy.
type Server struct {
	client    *client.Client
	cache     *cache.Cache
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server with the given client and cache.
func NewServer(apiClient *client.Client, exportCache *cache.Cache) *Server {
	s := &Server{
		client: apiClient,
		cache:  exportCache,
	}

	s.mcpServer = server.NewMCPServer(
		"workflowy",
		"1.0.0",
		server.WithLogging(),
	)

	s.registerTools()

	return s
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("search_nodes",
		mcp.WithDescription(
			"Search all Workflowy nodes by keyword. Matches against node name and note fields. "+
				"Returns matching nodes with their breadcrumb path for context."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query to match in node names and notes (case-insensitive substring match)"),
		),
		mcp.WithBoolean("completed",
			mcp.Description("Filter by completion status: true for completed only, false for uncompleted only (default: false)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
		),
	), s.handleSearchNodes)

	s.mcpServer.AddTool(mcp.NewTool("get_node",
		mcp.WithDescription("Get full details of a specific Workflowy node by its ID."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to retrieve"),
		),
	), s.handleGetNode)

	s.mcpServer.AddTool(mcp.NewTool("list_children",
		mcp.WithDescription(
			"List child nodes of a given parent. Nodes are returned sorted by priority. "+
				"Use a target key like 'home' or 'inbox' as parent_id, or omit for top-level nodes."),
		mcp.WithString("parent_id",
			mcp.Description("Parent node UUID, target key ('home', 'inbox'), or omit for top-level nodes"),
		),
	), s.handleListChildren)

	s.mcpServer.AddTool(mcp.NewTool("create_node",
		mcp.WithDescription("Create a new Workflowy node/bullet."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Text content of the node (supports markdown formatting)"),
		),
		mcp.WithString("parent_id",
			mcp.Description("Parent node UUID or target key ('home', 'inbox'). Omit for top-level."),
		),
		mcp.WithString("note",
			mcp.Description("Additional note content below the main text"),
		),
		mcp.WithString("layout_mode",
			mcp.Description("Display mode: 'bullets' (default), 'todo', 'h1', 'h2', 'h3', 'code-block', 'quote-block'"),
		),
		mcp.WithString("position",
			mcp.Description("Position among siblings: 'top' (default) or 'bottom'"),
		),
	), s.handleCreateNode)

	s.mcpServer.AddTool(mcp.NewTool("update_node",
		mcp.WithDescription("Update properties of an existing Workflowy node."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to update"),
		),
		mcp.WithString("name",
			mcp.Description("New text content for the node"),
		),
		mcp.WithString("note",
			mcp.Description("New note content"),
		),
		mcp.WithString("layout_mode",
			mcp.Description("New display mode: 'bullets', 'todo', 'h1', 'h2', 'h3', 'code-block', 'quote-block'"),
		),
	), s.handleUpdateNode)

	s.mcpServer.AddTool(mcp.NewTool("delete_node",
		mcp.WithDescription("Delete a Workflowy node by its ID."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to delete"),
		),
	), s.handleDeleteNode)

	s.mcpServer.AddTool(mcp.NewTool("move_node",
		mcp.WithDescription("Move a Workflowy node to a different parent."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to move"),
		),
		mcp.WithString("parent_id",
			mcp.Description("Destination parent UUID or target key"),
		),
		mcp.WithString("position",
			mcp.Description("Position at destination: 'top' or 'bottom'"),
		),
	), s.handleMoveNode)

	s.mcpServer.AddTool(mcp.NewTool("complete_node",
		mcp.WithDescription("Mark a Workflowy node as completed."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to complete"),
		),
	), s.handleCompleteNode)

	s.mcpServer.AddTool(mcp.NewTool("uncomplete_node",
		mcp.WithDescription("Mark a Workflowy node as not completed (uncomplete it)."),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The UUID of the node to uncomplete"),
		),
	), s.handleUncompleteNode)

	s.mcpServer.AddTool(mcp.NewTool("list_targets",
		mcp.WithDescription("List all Workflowy targets (system locations like 'home'/'inbox' and user shortcuts)."),
	), s.handleListTargets)
}

package server

import (
	"github.com/jbeshir/mcp-servers/wanikani/internal/client"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for WaniKani.
type Server struct {
	client    *client.Client
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server with the given client.
func NewServer(apiClient *client.Client) *Server {
	s := &Server{
		client: apiClient,
	}

	s.mcpServer = server.NewMCPServer(
		"wanikani",
		"0.1.0",
		server.WithLogging(),
	)

	s.registerTools()

	return s
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error {
	return server.ServeStdio(s.mcpServer)
}

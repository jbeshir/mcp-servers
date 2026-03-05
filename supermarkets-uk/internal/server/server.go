// Package server provides the MCP server for supermarket product search.
package server

import (
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for supermarket product search.
type Server struct {
	client    *client.Client
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server with the given client.
func NewServer(c *client.Client) *Server {
	s := &Server{
		client: c,
	}

	s.mcpServer = server.NewMCPServer(
		"supermarket",
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

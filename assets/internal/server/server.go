package server

import (
	"log"

	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for offline design assets (icons, illustrations, fonts).
type Server struct {
	mcpServer *server.MCPServer
	catalog   *catalog.Catalog
}

// NewServer creates a new MCP server, loading the embedded asset catalog.
func NewServer() *Server {
	c, err := catalog.Load()
	if err != nil {
		log.Printf("failed to load asset catalog: %v", err)
		c = &catalog.Catalog{}
	}

	s := &Server{
		catalog: c,
	}

	s.mcpServer = server.NewMCPServer(
		"assets",
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

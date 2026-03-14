// Package server provides the MCP server for Amazon product search.
package server

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/jbeshir/mcp-servers/amazon-products/internal/scraper"
)

// Server is the MCP server for Amazon product search.
type Server struct {
	browser         *scraper.Browser
	defaultRegionID string
	mcpServer       *server.MCPServer
}

// NewServer creates a new MCP server.
func NewServer(browser *scraper.Browser, defaultRegionID string) *Server {
	s := &Server{browser: browser, defaultRegionID: defaultRegionID}
	s.mcpServer = server.NewMCPServer(
		"amazon",
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

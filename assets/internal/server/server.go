package server

import (
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for offline design assets (icons, illustrations, fonts). It serves assets
// through a provider registry; the catalog is retained for the list_asset_sources tool.
type Server struct {
	mcpServer *server.MCPServer
	registry  *assetcore.Registry
	catalog   *catalog.Catalog
}

// NewServer creates a new MCP server backed by the given provider registry and catalog. The registry
// and catalog are built during wiring (config.Setup) and treated read-only.
func NewServer(registry *assetcore.Registry, cat *catalog.Catalog) *Server {
	s := &Server{
		registry: registry,
		catalog:  cat,
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

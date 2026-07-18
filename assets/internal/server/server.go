package server

import (
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/packstore"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for offline design assets (icons, illustrations, fonts, photos, textures,
// 3D models). It serves assets entirely through the provider registry; providers self-describe their
// sources for discovery.
type Server struct {
	mcpServer *server.MCPServer
	registry  *assetcore.Registry
	outputDir string
	packStore packstore.Store
}

// NewServer creates a new MCP server backed by the given provider registry and output directory. The
// registry is built during wiring (config.Setup) and treated read-only; outputDir is the resolved
// directory rendered assets are written to.
func NewServer(registry *assetcore.Registry, outputDir string, stores ...packstore.Store) *Server {
	s := &Server{
		registry:  registry,
		outputDir: outputDir,
	}
	if len(stores) > 0 {
		s.packStore = stores[0]
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

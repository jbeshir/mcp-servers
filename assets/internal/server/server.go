package server

import (
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for offline design assets (icons, illustrations, fonts, photos, textures,
// 3D models). It serves assets entirely through the provider registry; providers self-describe their
// sources for discovery.
type Server struct {
	mcpServer *server.MCPServer
	registry  *assetcore.Registry
	outputDir string
	packStore assetcore.PackStore
}

// NewServer creates a new MCP server backed by the given provider registry, output directory, and
// pack store. The dependencies are built during wiring (config.Setup) and treated read-only.
func NewServer(registry *assetcore.Registry, outputDir string, packStore assetcore.PackStore) *Server {
	s := &Server{
		registry:  registry,
		outputDir: outputDir,
		packStore: packStore,
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

package server

import (
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for offline design assets (icons, illustrations, fonts). It serves assets
// entirely through the provider registry; providers self-describe their sources for discovery.
type Server struct {
	mcpServer *server.MCPServer
	registry  *assetcore.Registry
	outputDir string
}

// NewServer creates a new MCP server backed by the given provider registry and output directory. The
// registry is built during wiring (config.Setup) and treated read-only; outputDir is the resolved
// directory rendered assets are written to.
func NewServer(registry *assetcore.Registry, outputDir string) *Server {
	s := &Server{
		registry:  registry,
		outputDir: outputDir,
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

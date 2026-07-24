package server

import (
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/service"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer *mcpserver.MCPServer
	service   *service.Service
}

func New(service *service.Service) *Server {
	s := &Server{service: service}
	s.mcpServer = mcpserver.NewMCPServer(
		"discord-emojigen",
		"0.1.0",
		mcpserver.WithLogging(),
	)
	s.registerTools()
	return s
}

func (s *Server) Run() error {
	return mcpserver.ServeStdio(s.mcpServer)
}

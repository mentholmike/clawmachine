package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mentholmike/clawmachine/internal/config"
	"github.com/mentholmike/clawmachine/internal/wagmios"
)

// Server is the ClawMachine MCP server (single-instance mode).
type Server struct {
	mcpServer *server.MCPServer
	cfg       config.Config
	client    *wagmios.Client
	scopes    []string
}

// NewServer creates a new ClawMachine MCP server for a single WAGMIOS instance.
func NewServer(cfg config.Config) (*Server, error) {
	client := wagmios.NewClient(cfg.APIURL, cfg.APIKey)

	authStatus, err := client.GetAuthStatus()
	if err != nil {
		return nil, fmt.Errorf("check API key status: %w", err)
	}
	if !authStatus.HasKey {
		return nil, fmt.Errorf("API key not recognized by WAGMIOS")
	}

	scopes := authStatus.Meta.Scopes
	log.Printf("API key: %s (scopes: %s)", authStatus.Meta.KeyPrefix, strings.Join(scopes, ", "))

	s := &Server{
		cfg:    cfg,
		client: client,
		scopes: scopes,
	}

	s.mcpServer = server.NewMCPServer(
		"ClawMachine",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	// Register all tools without host parameter (single-instance)
	getClient := func(_ mcp.CallToolRequest, _ string) (*wagmios.Client, error) {
		return s.client, nil
	}
	hasScope := func(scope string) bool { return s.hasScope(scope) }
	registerAllTools(s.mcpServer, getClient, hasScope, nil)

	return s, nil
}

// hasScope checks if the API key has a specific scope.
func (s *Server) hasScope(scope string) bool {
	for _, sc := range s.scopes {
		if sc == scope {
			return true
		}
	}
	return false
}

// Run starts the MCP server with the configured transport.
func (s *Server) Run(ctx context.Context) error {
	return runTransport(s.mcpServer, s.cfg.Transport, s.cfg.SSEAddr, s.cfg.SSEBaseURL)
}

// runTransport starts an MCP server with the given transport settings.
func runTransport(srv *server.MCPServer, transport, sseAddr, sseBaseURL string) error {
	switch transport {
	case "stdio":
		log.Println("Starting ClawMachine MCP server (stdio)")
		return server.ServeStdio(srv)
	case "sse":
		log.Printf("Starting ClawMachine MCP server (SSE) on %s", sseAddr)
		sseServer := server.NewSSEServer(srv,
			server.WithBaseURL(sseBaseURL),
			server.WithKeepAlive(true),
			server.WithKeepAliveInterval(30*time.Second),
		)
		return sseServer.Start(sseAddr)
	default:
		return fmt.Errorf("unknown transport: %s (use 'stdio' or 'sse')", transport)
	}
}

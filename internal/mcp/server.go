package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
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
	version   string
}

// NewServer creates a new ClawMachine MCP server for a single WAGMIOS instance.
func NewServer(cfg config.Config, version string) (*Server, error) {
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
		cfg:     cfg,
		client:  client,
		scopes:  scopes,
		version: version,
	}

	s.mcpServer = server.NewMCPServer(
		"ClawMachine",
		version,
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
	return runTransport(ctx, s.mcpServer, s.cfg.Transport, s.cfg.SSEAddr, s.cfg.SSEBaseURL, s.version)
}

// runTransport starts an MCP server with the given transport settings.
func runTransport(ctx context.Context, srv *server.MCPServer, transport, sseAddr, sseBaseURL, version string) error {
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
		// Wrap SSE server with a health endpoint on a custom mux
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status":    "healthy",
				"version":   version,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				log.Printf("healthz encode error: %v", err)
			}
		})
		mux.Handle("/", sseServer)
		httpServer := &http.Server{
			Addr:         sseAddr,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0, // SSE: disable write timeout — keep-alive handles liveness
			IdleTimeout:  120 * time.Second,
		}
		// Bind synchronously to fail fast on port conflicts
		ln, err := net.Listen("tcp", sseAddr)
		if err != nil {
			return fmt.Errorf("bind SSE address %s: %w", sseAddr, err)
		}
		go func() {
			if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.Printf("SSE server error: %v", err)
			}
		}()
		// Block until context is cancelled, then graceful shutdown
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			// SSE connections are long-lived; timeout is expected if clients are active.
			// Log but don't treat as fatal — force-close remaining connections.
			log.Printf("SSE server shutdown timed out (expected with active clients): %v", err)
			httpServer.Close()
		}
		return nil
	default:
		return fmt.Errorf("unknown transport: %s (use 'stdio' or 'sse')", transport)
	}
}


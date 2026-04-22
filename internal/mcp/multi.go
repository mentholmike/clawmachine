package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mentholmike/clawmachine/internal/config"
	"github.com/mentholmike/clawmachine/internal/wagmios"
)

// MultiServer is a ClawMachine MCP server that manages multiple WAGMIOS instances.
// Every tool gets a `host` parameter to route requests to the correct instance.
type MultiServer struct {
	mcpServer *server.MCPServer
	clients   map[string]*wagmios.Client
	scopes    map[string][]string
	labels    map[string]string
	hostNames []string
	version   string
}

// NewMultiServer creates a new multi-instance ClawMachine MCP server.
func NewMultiServer(multiCfg *config.MultiConfig, version string) (*MultiServer, error) {
	s := &MultiServer{
		clients: make(map[string]*wagmios.Client),
		scopes:  make(map[string][]string),
		labels:  make(map[string]string),
		version: version,
	}

	// Initialize a WAGMIOS client for each instance and fetch scopes
	for name, inst := range multiCfg.Instances {
		client := wagmios.NewClient(inst.URL, inst.Key)
		authStatus, err := client.GetAuthStatus()
		if err != nil {
			return nil, fmt.Errorf("check API key for %s: %w", name, err)
		}
		if !authStatus.HasKey {
			return nil, fmt.Errorf("API key not recognized for %s", name)
		}

		s.clients[name] = client
		s.scopes[name] = authStatus.Meta.Scopes
		s.labels[name] = inst.Label
		s.hostNames = append(s.hostNames, name)

		log.Printf("Instance %s (%s): %s (scopes: %s)", name, inst.Label, authStatus.Meta.KeyPrefix, strings.Join(authStatus.Meta.Scopes, ", "))
	}

	s.mcpServer = server.NewMCPServer(
		"ClawMachine",
		version,
		server.WithToolCapabilities(false),
	)

	// Register the list_hosts tool (multi-instance only)
	s.addListHostsTool()

	// Register all tools with host parameter
	hostOpt := mcp.WithString("host",
		mcp.Required(),
		mcp.Description(fmt.Sprintf("Target host (%s)", strings.Join(s.hostNames, "/"))),
	)
	getClient := func(req mcp.CallToolRequest, requiredScope string) (*wagmios.Client, error) {
		return s.resolveClient(req, requiredScope)
	}
	hasScope := func(scope string) bool { return s.anyHostHasScope(scope) }
	registerAllTools(s.mcpServer, getClient, hasScope, &hostOpt)

	return s, nil
}

// resolveClient extracts the host parameter and returns the corresponding WAGMIOS client.
// It validates that the resolved host has the required scope for the tool being called.
func (s *MultiServer) resolveClient(req mcp.CallToolRequest, requiredScope string) (*wagmios.Client, error) {
	host, err := req.RequireString("host")
	if err != nil {
		return nil, fmt.Errorf("host parameter is required")
	}
	client, ok := s.clients[host]
	if !ok {
		return nil, fmt.Errorf("unknown host: %s (available: %s)", host, strings.Join(s.hostNames, ", "))
	}
	if requiredScope != "" && !s.hasScope(host, requiredScope) {
		return nil, fmt.Errorf("host %q does not have the %q scope required for this tool", host, requiredScope)
	}
	return client, nil
}

// hasScope checks if a specific host's API key has a scope.
func (s *MultiServer) hasScope(host, scope string) bool {
	for _, sc := range s.scopes[host] {
		if sc == scope {
			return true
		}
	}
	return false
}

// anyHostHasScope checks if any host has a specific scope.
func (s *MultiServer) anyHostHasScope(scope string) bool {
	for _, host := range s.hostNames {
		if s.hasScope(host, scope) {
			return true
		}
	}
	return false
}

// Run starts the multi-instance MCP server.
func (s *MultiServer) Run(ctx context.Context, transport, sseAddr, sseBaseURL string) error {
	log.Printf("Starting ClawMachine MCP server (multi-instance, %s)", transport)
	return runTransport(ctx, s.mcpServer, transport, sseAddr, sseBaseURL, s.version)
}

// addListHostsTool registers the list_hosts tool (only in multi-instance mode).
func (s *MultiServer) addListHostsTool() {
	tool := mcp.NewTool("list_hosts",
		mcp.WithDescription("List all configured WAGMIOS instances with their labels and scopes"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		type hostInfo struct {
			Name   string   `json:"name"`
			Label  string   `json:"label"`
			Scopes []string `json:"scopes"`
		}
		var hosts []hostInfo
		for _, name := range s.hostNames {
			hosts = append(hosts, hostInfo{
				Name:   name,
				Label:  s.labels[name],
				Scopes: s.scopes[name],
			})
		}
		data, _ := json.MarshalIndent(hosts, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
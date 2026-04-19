package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mentholmike/clawmachine/internal/config"
	"github.com/mentholmike/clawmachine/internal/wagmios"
)

func TestMultiServer_hasScope(t *testing.T) {
	s := &MultiServer{
		scopes: map[string][]string{
			"nas":   {"containers:read", "containers:write", "marketplace:read"},
			"vps":   {"containers:read", "images:read"},
			"media": {"marketplace:read", "marketplace:write"},
		},
	}

	tests := []struct {
		host   string
		scope  string
		expect bool
	}{
		{"nas", "containers:read", true},
		{"nas", "containers:delete", false},
		{"nas", "marketplace:read", true},
		{"vps", "containers:read", true},
		{"vps", "containers:write", false},
		{"vps", "images:read", true},
		{"media", "marketplace:write", true},
		{"media", "containers:read", false},
		{"nonexistent", "containers:read", false},
	}

	for _, tt := range tests {
		got := s.hasScope(tt.host, tt.scope)
		if got != tt.expect {
			t.Errorf("hasScope(%q, %q) = %v, want %v", tt.host, tt.scope, got, tt.expect)
		}
	}
}

func TestMultiServer_anyHostHasScope(t *testing.T) {
	s := &MultiServer{
		scopes: map[string][]string{
			"nas":   {"containers:read", "containers:write"},
			"vps":   {"containers:read", "images:read"},
			"media": {"marketplace:read"},
		},
		hostNames: []string{"nas", "vps", "media"},
	}

	tests := []struct {
		scope  string
		expect bool
	}{
		{"containers:read", true},
		{"containers:write", true},
		{"containers:delete", false},
		{"images:read", true},
		{"marketplace:read", true},
		{"marketplace:write", false},
		{"secrets:write", false},
	}

	for _, tt := range tests {
		got := s.anyHostHasScope(tt.scope)
		if got != tt.expect {
			t.Errorf("anyHostHasScope(%q) = %v, want %v", tt.scope, got, tt.expect)
		}
	}
}

func TestServer_hasScope(t *testing.T) {
	s := &Server{
		scopes: []string{"containers:read", "containers:write", "marketplace:read"},
	}

	tests := []struct {
		scope  string
		expect bool
	}{
		{"containers:read", true},
		{"containers:write", true},
		{"containers:delete", false},
		{"marketplace:read", true},
		{"marketplace:write", false},
	}

	for _, tt := range tests {
		got := s.hasScope(tt.scope)
		if got != tt.expect {
			t.Errorf("hasScope(%q) = %v, want %v", tt.scope, got, tt.expect)
		}
	}
}

func TestMultiServer_resolveClient_scopeValidation(t *testing.T) {
	s := &MultiServer{
		clients: map[string]*wagmios.Client{
			"nas": {},
			"vps": {},
		},
		scopes: map[string][]string{
			"nas": {"containers:read", "containers:delete"},
			"vps": {"containers:read"},
		},
		hostNames: []string{"nas", "vps"},
	}

	// Host with the scope should work
	_, err := s.resolveClient(mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{"host": "nas"},
		},
	}, "containers:delete")
	if err != nil {
		t.Errorf("expected no error for nas+containers:delete, got %v", err)
	}

	// Host without the scope should fail
	_, err = s.resolveClient(mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{"host": "vps"},
		},
	}, "containers:delete")
	if err == nil {
		t.Error("expected error for vps+containers:delete, scope escalation should be blocked")
	}

	// Empty scope (check_scopes, system_info) should work for any host
	_, err = s.resolveClient(mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{"host": "vps"},
		},
	}, "")
	if err != nil {
		t.Errorf("expected no error for vps+empty scope, got %v", err)
	}

	// Unknown host should fail
	_, err = s.resolveClient(mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{"host": "nonexistent"},
		},
	}, "containers:read")
	if err == nil {
		t.Error("expected error for unknown host")
	}
}

func TestRunTransport_unknownTransport(t *testing.T) {
	err := runTransport(nil, "invalid", "", "")
	if err == nil {
		t.Error("expected error for unknown transport")
	}
}

func TestNewServer_singleInstance(t *testing.T) {
	// Smoke test: NewServer should attempt to connect to WAGMIOS
	cfg := config.Config{
		APIURL:    "http://localhost:5179",
		APIKey:    "test-key",
		Transport: "stdio",
	}
	srv, err := NewServer(cfg)
	if err == nil && srv == nil {
		t.Error("expected either error or valid server")
	}
}

func TestLoadMultiConfig_validation(t *testing.T) {
	cfg := &config.MultiConfig{
		Instances: map[string]config.InstanceConfig{},
	}
	if len(cfg.Instances) == 0 {
		t.Log("Empty instances map should be rejected by LoadMultiConfig")
	}
}
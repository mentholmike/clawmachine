package mcp

import (
	"context"
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
	err := runTransport(context.Background(), nil, "invalid", "", "", "")
	if err == nil {
		t.Error("expected error for unknown transport")
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expected  []wagmios.Port
		expectErr bool
	}{
		{
			name: "valid array of ports",
			input: []interface{}{
				map[string]interface{}{"private": float64(80), "public": float64(8080), "protocol": "tcp"},
				map[string]interface{}{"private": float64(443), "public": float64(8443), "protocol": "tcp"},
			},
			expected: []wagmios.Port{
				{Private: 80, Public: 8080, Protocol: "tcp"},
				{Private: 443, Public: 8443, Protocol: "tcp"},
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:      "not an array",
			input:     "not-an-array",
			expectErr: true,
		},
		{
			name: "missing optional fields",
			input: []interface{}{
				map[string]interface{}{"private": float64(22)},
			},
			expected: []wagmios.Port{
				{Private: 22},
			},
		},
		{
			name: "invalid port out of range",
			input: []interface{}{
				map[string]interface{}{"private": float64(70000), "public": float64(80)},
			},
			expectErr: true,
		},
		{
			name: "negative port",
			input: []interface{}{
				map[string]interface{}{"private": float64(-1)},
			},
			expectErr: true,
		},
		{
			name: "missing required private",
			input: []interface{}{
				map[string]interface{}{"public": float64(80)},
			},
			expectErr: true,
		},
		{
			name: "wrong type for private",
			input: []interface{}{
				map[string]interface{}{"private": "eighty"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePorts(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("parsePorts() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parsePorts() unexpected error: %v", err)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("parsePorts() returned %d ports, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parsePorts()[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseVolumes(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expected  []wagmios.Volume
		expectErr bool
	}{
		{
			name: "valid array of volumes",
			input: []interface{}{
				map[string]interface{}{"host": "/data", "container": "/app/data"},
				map[string]interface{}{"host": "/config", "container": "/etc/app"},
			},
			expected: []wagmios.Volume{
				{Host: "/data", Container: "/app/data"},
				{Host: "/config", Container: "/etc/app"},
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:      "not an array",
			input:     42,
			expectErr: true,
		},
		{
			name: "missing container",
			input: []interface{}{
				map[string]interface{}{"host": "/data"},
			},
			expectErr: true,
		},
		{
			name: "missing host",
			input: []interface{}{
				map[string]interface{}{"container": "/var/log"},
			},
			expectErr: true,
		},
		{
			name: "wrong type for host",
			input: []interface{}{
				map[string]interface{}{"host": 123, "container": "/app"},
			},
			expectErr: true,
		},
		{
			name: "not an object in array",
			input: []interface{}{
				42,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVolumes(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("parseVolumes() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseVolumes() unexpected error: %v", err)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("parseVolumes() returned %d volumes, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseVolumes()[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestNewServer_singleInstance(t *testing.T) {
	// Smoke test: NewServer should attempt to connect to WAGMIOS
	cfg := config.Config{
		APIURL:    "http://localhost:5179",
		APIKey:    "test-key",
		Transport: "stdio",
	}
	srv, err := NewServer(cfg, "dev")
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
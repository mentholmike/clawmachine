package config

// Config holds the ClawMachine server configuration.
type Config struct {
	APIURL     string // WAGMIOS backend URL (e.g. http://localhost:5179)
	APIKey     string // WAGMIOS API key (X-API-Key header)
	Transport  string // MCP transport mode: "stdio" or "sse"
	SSEAddr    string // Address for SSE transport (default :8080)
	SSEBaseURL string // Base URL for SSE transport (e.g. http://localhost:8080) — needed for remote clients
}
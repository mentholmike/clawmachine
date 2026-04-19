package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mentholmike/clawmachine/internal/config"
	"github.com/mentholmike/clawmachine/internal/mcp"
)

func main() {
	apiURL := flag.String("api-url", "", "WAGMIOS API base URL (e.g. http://localhost:5179)")
	apiKey := flag.String("api-key", "", "WAGMIOS API key (X-API-Key header value)")
	transport := flag.String("transport", "stdio", "MCP transport: stdio or sse")
	sseAddr := flag.String("sse-addr", ":8080", "Address for SSE transport (when transport=sse)")
	flag.Parse()

	// Config can also come from env vars
	if *apiURL == "" {
		*apiURL = os.Getenv("WAGMIOS_API_URL")
	}
	if *apiKey == "" {
		*apiKey = os.Getenv("WAGMIOS_API_KEY")
	}

	if *apiURL == "" || *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: WAGMIOS API URL and key are required.")
		fmt.Fprintln(os.Stderr, "Use -api-url and -api-key flags, or WAGMIOS_API_URL and WAGMIOS_API_KEY env vars.")
		os.Exit(1)
	}

	cfg := config.Config{
		APIURL:    *apiURL,
		APIKey:    *apiKey,
		Transport: *transport,
		SSEAddr:   *sseAddr,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	server, err := mcp.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
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
	// Single-instance flags
	apiURL := flag.String("api-url", "", "WAGMIOS API base URL (single-instance mode)")
	apiKey := flag.String("api-key", "", "WAGMIOS API key (single-instance mode)")

	// Multi-instance flag
	configFile := flag.String("config", "", "Path to multi-instance config JSON (enables multi-instance mode)")

	// Transport flags
	transport := flag.String("transport", "stdio", "MCP transport: stdio or sse")
	sseAddr := flag.String("sse-addr", ":8080", "Address for SSE transport")
	sseBaseURL := flag.String("sse-base-url", "", "Base URL for SSE transport (needed for remote clients)")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *configFile != "" {
		// Multi-instance mode
		multiCfg, err := config.LoadMultiConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		server, err := mcp.NewMultiServer(multiCfg, *transport, *sseAddr, *sseBaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := server.Run(ctx, *transport, *sseAddr, *sseBaseURL); err != nil {
			log.Fatalf("Server error: %v", err)
		}
		return
	}

	// Single-instance mode
	url := *apiURL
	key := *apiKey
	if url == "" {
		url = os.Getenv("WAGMIOS_API_URL")
	}
	if key == "" {
		key = os.Getenv("WAGMIOS_API_KEY")
	}
	if url == "" || key == "" {
		fmt.Fprintln(os.Stderr, "Error: WAGMIOS API URL and key are required.")
		fmt.Fprintln(os.Stderr, "Use -api-url and -api-key, -config for multi-instance, or set WAGMIOS_API_URL/WAGMIOS_API_KEY env vars.")
		os.Exit(1)
	}

	cfg := config.Config{
		APIURL:     url,
		APIKey:     key,
		Transport:  *transport,
		SSEAddr:    *sseAddr,
		SSEBaseURL: *sseBaseURL,
	}

	server, err := mcp.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
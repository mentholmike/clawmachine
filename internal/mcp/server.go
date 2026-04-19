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

// Server is the ClawMachine MCP server.
type Server struct {
	mcpServer *server.MCPServer
	cfg       config.Config
	client    *wagmios.Client
	scopes    []string
}

// NewServer creates a new ClawMachine MCP server.
func NewServer(cfg config.Config) (*Server, error) {
	client := wagmios.NewClient(cfg.APIURL, cfg.APIKey)

	// Fetch scopes to determine which tools to expose
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

	s.registerTools()
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

// registerTools registers MCP tools based on available scopes.
func (s *Server) registerTools() {
	// Always available: check_scopes
	s.addCheckScopesTool()

	// System tools (available to any key)
	s.addSystemInfoTool()
	s.addSystemMetricsTool()

	// Container tools
	if s.hasScope("containers:read") {
		s.addListContainersTool()
		s.addContainerLogsTool()
		s.addContainerConfigTool()
	}
	if s.hasScope("containers:write") {
		s.addStartContainerTool()
		s.addStopContainerTool()
		s.addRestartContainerTool()
		s.addCreateContainerTool()
	}
	if s.hasScope("containers:delete") {
		s.addDeleteContainerTool()
	}

	// Image tools
	if s.hasScope("images:read") {
		s.addListImagesTool()
	}
	if s.hasScope("images:write") {
		s.addPullImageTool()
		s.addDeleteImageTool()
	}

	// Marketplace tools
	if s.hasScope("marketplace:read") {
		s.addBrowseMarketplaceTool()
		s.addGetMarketplaceAppTool()
		s.addListInstalledAppsTool()
	}
	if s.hasScope("marketplace:write") {
		s.addInstallMarketplaceAppTool()
		s.addStartMarketplaceAppTool()
	}
}

// Run starts the MCP server with the configured transport.
func (s *Server) Run(ctx context.Context) error {
	switch s.cfg.Transport {
	case "stdio":
		log.Println("Starting ClawMachine MCP server (stdio)")
		return server.ServeStdio(s.mcpServer)
	case "sse":
		log.Printf("Starting ClawMachine MCP server (SSE) on %s", s.cfg.SSEAddr)
		sseServer := server.NewSSEServer(s.mcpServer)
		return sseServer.Start(s.cfg.SSEAddr)
	default:
		return fmt.Errorf("unknown transport: %s (use 'stdio' or 'sse')", s.cfg.Transport)
	}
}

// --- Tool Registration ---

func (s *Server) addCheckScopesTool() {
	tool := mcp.NewTool("check_scopes",
		mcp.WithDescription("Check the current API key's status, label, and granted scopes"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := s.client.GetAuthStatus()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to check scopes: %v", err)), nil
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addSystemInfoTool() {
	tool := mcp.NewTool("system_info",
		mcp.WithDescription("Get Docker version, API version, and system information from the WAGMIOS host"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		info, err := s.client.GetSystemInfo()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get system info: %v", err)), nil
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addSystemMetricsTool() {
	tool := mcp.NewTool("system_metrics",
		mcp.WithDescription("Get CPU, memory, disk usage, and container counts from the WAGMIOS host"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		metrics, err := s.client.GetSystemMetrics()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get system metrics: %v", err)), nil
		}
		data, _ := json.MarshalIndent(metrics, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addListContainersTool() {
	tool := mcp.NewTool("list_containers",
		mcp.WithDescription("List all Docker containers (running and stopped)"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		containers, err := s.client.ListContainers()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list containers: %v", err)), nil
		}
		data, _ := json.MarshalIndent(containers, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addContainerLogsTool() {
	tool := mcp.NewTool("container_logs",
		mcp.WithDescription("Get log output from a container"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name"),
		),
		mcp.WithNumber("tail",
			mcp.Description("Number of lines to return from the end (default 100)"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		tail := 100
		if args, ok := req.Params.Arguments.(map[string]interface{}); ok {
			if t, ok := args["tail"]; ok {
				if tf, ok := t.(float64); ok {
					tail = int(tf)
				}
			}
		}
		logs, err := s.client.GetContainerLogs(id, tail)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get container logs: %v", err)), nil
		}
		return mcp.NewToolResultText(logs), nil
	})
}

func (s *Server) addContainerConfigTool() {
	tool := mcp.NewTool("container_config",
		mcp.WithDescription("Get the full configuration of a container (environment, volumes, ports, etc.)"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config, err := s.client.GetContainerConfig(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get container config: %v", err)), nil
		}
		return mcp.NewToolResultText(string(config)), nil
	})
}

func (s *Server) addStartContainerTool() {
	tool := mcp.NewTool("start_container",
		mcp.WithDescription("Start a stopped container"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.StartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s started", id)), nil
	})
}

func (s *Server) addStopContainerTool() {
	tool := mcp.NewTool("stop_container",
		mcp.WithDescription("Stop a running container"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.StopContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to stop container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s stopped", id)), nil
	})
}

func (s *Server) addRestartContainerTool() {
	tool := mcp.NewTool("restart_container",
		mcp.WithDescription("Restart a container"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.RestartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to restart container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s restarted", id)), nil
	})
}

func (s *Server) addCreateContainerTool() {
	tool := mcp.NewTool("create_container",
		mcp.WithDescription("Create a new Docker container"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Docker image (e.g. nginx:alpine)"),
		),
		mcp.WithString("name",
			mcp.Description("Container name"),
		),
		mcp.WithObject("env",
			mcp.Description("Environment variables as key-value pairs"),
		),
		mcp.WithObject("ports",
			mcp.Description("Port mappings (array of {private, public, protocol})"),
		),
		mcp.WithObject("volumes",
			mcp.Description("Volume mounts (array of {host, container})"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		image, err := req.RequireString("image")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		createReq := wagmios.CreateContainerRequest{Image: image}

		if args, ok := req.Params.Arguments.(map[string]interface{}); ok {
			if name, ok := args["name"]; ok {
				if n, ok := name.(string); ok {
					createReq.Name = n
				}
			}

			if env, ok := args["env"]; ok {
				if e, ok := env.(map[string]interface{}); ok {
					createReq.Env = make(map[string]string)
					for k, v := range e {
						if vs, ok := v.(string); ok {
							createReq.Env[k] = vs
						}
					}
				}
			}
		}

		container, err := s.client.CreateContainer(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create container: %v", err)), nil
		}
		data, _ := json.MarshalIndent(container, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addDeleteContainerTool() {
	tool := mcp.NewTool("delete_container",
		mcp.WithDescription("Delete a container permanently. This is irreversible — confirm with the user before calling."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Container ID or name to delete"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.DeleteContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s deleted", id)), nil
	})
}

func (s *Server) addListImagesTool() {
	tool := mcp.NewTool("list_images",
		mcp.WithDescription("List all Docker images on the host"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		images, err := s.client.ListImages()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list images: %v", err)), nil
		}
		data, _ := json.MarshalIndent(images, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addPullImageTool() {
	tool := mcp.NewTool("pull_image",
		mcp.WithDescription("Pull a Docker image from a registry"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Image name (e.g. nginx:alpine)"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		image, err := req.RequireString("image")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.PullImage(image); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to pull image: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s pulled", image)), nil
	})
}

func (s *Server) addDeleteImageTool() {
	tool := mcp.NewTool("delete_image",
		mcp.WithDescription("Delete a Docker image. This is irreversible — confirm with the user before calling."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Image ID to delete"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.DeleteImage(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete image: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s deleted", id)), nil
	})
}

func (s *Server) addBrowseMarketplaceTool() {
	tool := mcp.NewTool("browse_marketplace",
		mcp.WithDescription("Browse all available apps in the WAGMIOS marketplace (34+ self-hosted apps)"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		apps, err := s.client.BrowseMarketplace()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to browse marketplace: %v", err)), nil
		}
		data, _ := json.MarshalIndent(apps, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addGetMarketplaceAppTool() {
	tool := mcp.NewTool("get_marketplace_app",
		mcp.WithDescription("Get details for a specific marketplace app"),
		mcp.WithString("app_id",
			mcp.Required(),
			mcp.Description("App ID (e.g. jellyfin, nginx, ollama)"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		appID, err := req.RequireString("app_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		app, err := s.client.GetMarketplaceApp(appID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get marketplace app: %v", err)), nil
		}
		data, _ := json.MarshalIndent(app, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addListInstalledAppsTool() {
	tool := mcp.NewTool("list_installed_apps",
		mcp.WithDescription("List all apps installed via the WAGMIOS marketplace"),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		apps, err := s.client.ListInstalledApps()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list installed apps: %v", err)), nil
		}
		data, _ := json.MarshalIndent(apps, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addInstallMarketplaceAppTool() {
	tool := mcp.NewTool("install_app",
		mcp.WithDescription("Download and install a marketplace app (does not start it). Use start_app to start after installing."),
		mcp.WithString("app_id",
			mcp.Required(),
			mcp.Description("App ID to install (e.g. jellyfin, nginx, ollama)"),
		),
		mcp.WithString("container_name",
			mcp.Description("Custom container name (optional)"),
		),
		mcp.WithString("custom_name",
			mcp.Description("Custom app name (optional)"),
		),
		mcp.WithObject("environment",
			mcp.Description("Custom environment variables as key-value pairs (optional)"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		appID, err := req.RequireString("app_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		createReq := wagmios.MarketplaceCreateRequest{AppID: appID}

		if args, ok := req.Params.Arguments.(map[string]interface{}); ok {
			if name, ok := args["container_name"]; ok {
				if n, ok := name.(string); ok {
					createReq.ContainerName = n
				}
			}
			if name, ok := args["custom_name"]; ok {
				if n, ok := name.(string); ok {
					createReq.CustomName = n
				}
			}
			if env, ok := args["environment"]; ok {
				if e, ok := env.(map[string]interface{}); ok {
					createReq.Environment = make(map[string]string)
					for k, v := range e {
						if vs, ok := v.(string); ok {
							createReq.Environment[k] = vs
						}
					}
				}
			}
		}

		result, err := s.client.CreateMarketplaceApp(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to install app: %v", err)), nil
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *Server) addStartMarketplaceAppTool() {
	tool := mcp.NewTool("start_app",
		mcp.WithDescription("Start an installed marketplace app (pulls image and runs docker compose up)"),
		mcp.WithString("app_id",
			mcp.Required(),
			mcp.Description("App ID (e.g. jellyfin)"),
		),
		mcp.WithString("container_name",
			mcp.Required(),
			mcp.Description("Container name used during install"),
		),
		mcp.WithString("compose_path",
			mcp.Required(),
			mcp.Description("Compose file path returned by install_app"),
		),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		appID, err := req.RequireString("app_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		containerName, err := req.RequireString("container_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		composePath, err := req.RequireString("compose_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		startReq := wagmios.MarketplaceStartRequest{
			AppID:         appID,
			ContainerName: containerName,
			ComposePath:   composePath,
		}

		if err := s.client.StartMarketplaceApp(startReq); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start app: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("App %s started", appID)), nil
	})
}
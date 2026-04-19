package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mentholmike/clawmachine/internal/wagmios"
)

// clientGetter returns a WAGMIOS client for a tool call, validating the
// required scope if one is specified.
// In single-instance mode, it always returns the default client (scope already verified at registration).
// In multi-instance mode, it resolves the host parameter and checks the host's scopes.
type clientGetter func(req mcp.CallToolRequest, requiredScope string) (*wagmios.Client, error)

// registerAllTools registers all MCP tools using a client getter function.
// This is the single source of truth for tool definitions — no duplication.
//
// hostOpt, when non-nil, adds a "host" parameter to every tool (multi-instance mode).
// The *mcp.ToolOption nil-sentinel pattern is used because mcp.ToolOption is a
// function type — a nil pointer distinguishes "no host option" from an empty option.
func registerAllTools(mcpServer *server.MCPServer, getClient clientGetter, hasScope func(string) bool, hostOpt *mcp.ToolOption) {
	// Always available
	addCheckScopesTool(mcpServer, getClient, hostOpt)
	addSystemInfoTool(mcpServer, getClient, hostOpt)
	addSystemMetricsTool(mcpServer, getClient, hostOpt)

	// Container tools
	if hasScope("containers:read") {
		addListContainersTool(mcpServer, getClient, hostOpt)
		addContainerLogsTool(mcpServer, getClient, hostOpt)
		addContainerConfigTool(mcpServer, getClient, hostOpt)
	}
	if hasScope("containers:write") {
		addStartContainerTool(mcpServer, getClient, hostOpt)
		addStopContainerTool(mcpServer, getClient, hostOpt)
		addRestartContainerTool(mcpServer, getClient, hostOpt)
		addCreateContainerTool(mcpServer, getClient, hostOpt)
	}
	if hasScope("containers:delete") {
		addDeleteContainerTool(mcpServer, getClient, hostOpt)
	}

	// Image tools
	if hasScope("images:read") {
		addListImagesTool(mcpServer, getClient, hostOpt)
	}
	if hasScope("images:write") {
		addPullImageTool(mcpServer, getClient, hostOpt)
		addDeleteImageTool(mcpServer, getClient, hostOpt)
	}

	// Marketplace tools
	if hasScope("marketplace:read") {
		addBrowseMarketplaceTool(mcpServer, getClient, hostOpt)
		addGetMarketplaceAppTool(mcpServer, getClient, hostOpt)
		addListInstalledAppsTool(mcpServer, getClient, hostOpt)
	}
	if hasScope("marketplace:write") {
		addInstallMarketplaceAppTool(mcpServer, getClient, hostOpt)
		addStartMarketplaceAppTool(mcpServer, getClient, hostOpt)
	}
}

// --- Tool Implementations ---

func addCheckScopesTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Check the current API key's status, label, and granted scopes"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("check_scopes", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		status, err := client.GetAuthStatus()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to check scopes: %v", err)), nil
		}
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addSystemInfoTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Get Docker version, API version, and system information from the WAGMIOS host"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("system_info", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		info, err := client.GetSystemInfo()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get system info: %v", err)), nil
		}
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addSystemMetricsTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Get CPU, memory, disk usage, and container counts from the WAGMIOS host"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("system_metrics", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		metrics, err := client.GetSystemMetrics()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get system metrics: %v", err)), nil
		}
		data, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addListContainersTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("List all Docker containers (running and stopped)"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("list_containers", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		containers, err := client.ListContainers()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list containers: %v", err)), nil
		}
		data, err := json.MarshalIndent(containers, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addContainerLogsTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Get log output from a container"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
		mcp.WithNumber("tail", mcp.Description("Number of lines to return from the end (default 100)")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("container_logs", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		logs, err := client.GetContainerLogs(id, tail)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get container logs: %v", err)), nil
		}
		return mcp.NewToolResultText(logs), nil
	})
}

func addContainerConfigTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Get the full configuration of a container (environment, volumes, ports, etc.)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("container_config", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config, err := client.GetContainerConfig(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get container config: %v", err)), nil
		}
		return mcp.NewToolResultText(string(config)), nil
	})
}

func addStartContainerTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Start a stopped container"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("start_container", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.StartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s started", id)), nil
	})
}

func addStopContainerTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Stop a running container"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("stop_container", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.StopContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to stop container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s stopped", id)), nil
	})
}

func addRestartContainerTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Restart a container"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("restart_container", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.RestartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to restart container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s restarted", id)), nil
	})
}

func addCreateContainerTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Create a new Docker container"),
		mcp.WithString("image", mcp.Required(), mcp.Description("Docker image (e.g. nginx:alpine)")),
		mcp.WithString("name", mcp.Description("Container name")),
		mcp.WithObject("env", mcp.Description("Environment variables as key-value pairs")),
		mcp.WithObject("ports", mcp.Description("Port mappings (array of {private, public, protocol})")),
		mcp.WithObject("volumes", mcp.Description("Volume mounts (array of {host, container})")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("create_container", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		container, err := client.CreateContainer(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create container: %v", err)), nil
		}
		data, err := json.MarshalIndent(container, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addDeleteContainerTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Delete a container permanently. This is irreversible — confirm with the user before calling."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name to delete")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("delete_container", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "containers:delete")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.DeleteContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete container: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s deleted", id)), nil
	})
}

func addListImagesTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("List all Docker images on the host"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("list_images", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "images:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		images, err := client.ListImages()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list images: %v", err)), nil
		}
		data, err := json.MarshalIndent(images, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addPullImageTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Pull a Docker image from a registry"),
		mcp.WithString("image", mcp.Required(), mcp.Description("Image name (e.g. nginx:alpine)")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("pull_image", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "images:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		image, err := req.RequireString("image")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.PullImage(image); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to pull image: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s pulled", image)), nil
	})
}

func addDeleteImageTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Delete a Docker image. This is irreversible — confirm with the user before calling."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Image ID to delete")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("delete_image", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "images:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.DeleteImage(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete image: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s deleted", id)), nil
	})
}

func addBrowseMarketplaceTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Browse all available apps in the WAGMIOS marketplace (34+ self-hosted apps)"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("browse_marketplace", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		apps, err := client.BrowseMarketplace()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to browse marketplace: %v", err)), nil
		}
		data, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addGetMarketplaceAppTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Get details for a specific marketplace app"),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID (e.g. jellyfin, nginx, ollama)")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("get_marketplace_app", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		appID, err := req.RequireString("app_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		app, err := client.GetMarketplaceApp(appID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get marketplace app: %v", err)), nil
		}
		data, err := json.MarshalIndent(app, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addListInstalledAppsTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("List all apps installed via the WAGMIOS marketplace"),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("list_installed_apps", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		apps, err := client.ListInstalledApps()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list installed apps: %v", err)), nil
		}
		data, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addInstallMarketplaceAppTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Download and install a marketplace app (does not start it). Use start_app to start after installing."),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID to install (e.g. jellyfin, nginx, ollama)")),
		mcp.WithString("container_name", mcp.Description("Custom container name (optional)")),
		mcp.WithString("custom_name", mcp.Description("Custom app name (optional)")),
		mcp.WithObject("environment", mcp.Description("Custom environment variables as key-value pairs (optional)")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("install_app", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "marketplace:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		result, err := client.CreateMarketplaceApp(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to install app: %v", err)), nil
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

func addStartMarketplaceAppTool(s *server.MCPServer, getClient clientGetter, hostOpt *mcp.ToolOption) {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Start an installed marketplace app (pulls image and runs docker compose up)"),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID (e.g. jellyfin)")),
		mcp.WithString("container_name", mcp.Required(), mcp.Description("Container name used during install")),
		mcp.WithString("compose_path", mcp.Required(), mcp.Description("Compose file path returned by install_app")),
	}
	if hostOpt != nil {
		opts = append(opts, *hostOpt)
	}
	tool := mcp.NewTool("start_app", opts...)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := getClient(req, "marketplace:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		if err := client.StartMarketplaceApp(startReq); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start app: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("App %s started", appID)), nil
	})
}
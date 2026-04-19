package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mentholmike/clawmachine/internal/config"
	"github.com/mentholmike/clawmachine/internal/wagmios"
)

// MultiServer is a ClawMachine MCP server that manages multiple WAGMIOS instances.
// Every tool gets a `host` parameter to route requests to the correct instance.
type MultiServer struct {
	mcpServer *server.MCPServer
	clients   map[string]*wagmios.Client   // host name → client
	scopes    map[string][]string           // host name → scopes
	labels    map[string]string             // host name → human label
	hostNames []string                      // ordered list for consistent enumeration
}

// NewMultiServer creates a new multi-instance ClawMachine MCP server.
func NewMultiServer(multiCfg *config.MultiConfig, transport, sseAddr, sseBaseURL string) (*MultiServer, error) {
	s := &MultiServer{
		clients: make(map[string]*wagmios.Client),
		scopes:  make(map[string][]string),
		labels:  make(map[string]string),
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
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	s.registerTools()
	return s, nil
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
	switch transport {
	case "stdio":
		log.Println("Starting ClawMachine MCP server (multi-instance, stdio)")
		return server.ServeStdio(s.mcpServer)
	case "sse":
		log.Printf("Starting ClawMachine MCP server (multi-instance, SSE) on %s", sseAddr)
		sseServer := server.NewSSEServer(s.mcpServer,
			server.WithBaseURL(sseBaseURL),
			server.WithKeepAlive(true),
			server.WithKeepAliveInterval(30*time.Second),
		)
		return sseServer.Start(sseAddr)
	default:
		return fmt.Errorf("unknown transport: %s", transport)
	}
}

// registerTools registers tools with a `host` parameter for multi-instance routing.
// A tool is registered if ANY host has the required scope for it.
func (s *MultiServer) registerTools() {
	s.addListHostsTool()

	if s.anyHostHasScope("containers:read") {
		s.addListContainersTool()
		s.addContainerLogsTool()
		s.addContainerConfigTool()
	}
	if s.anyHostHasScope("containers:write") {
		s.addStartContainerTool()
		s.addStopContainerTool()
		s.addRestartContainerTool()
		s.addCreateContainerTool()
	}
	if s.anyHostHasScope("containers:delete") {
		s.addDeleteContainerTool()
	}
	if s.anyHostHasScope("images:read") {
		s.addListImagesTool()
	}
	if s.anyHostHasScope("images:write") {
		s.addPullImageTool()
		s.addDeleteImageTool()
	}
	if s.anyHostHasScope("marketplace:read") {
		s.addBrowseMarketplaceTool()
		s.addGetMarketplaceAppTool()
		s.addListInstalledAppsTool()
	}
	if s.anyHostHasScope("marketplace:write") {
		s.addInstallMarketplaceAppTool()
		s.addStartMarketplaceAppTool()
	}
}

// getClient resolves the host parameter to a WAGMIOS client, checking scope.
func (s *MultiServer) getClient(args map[string]interface{}, scope string) (*wagmios.Client, error) {
	host, ok := args["host"].(string)
	if !ok || host == "" {
		return nil, fmt.Errorf("host parameter is required")
	}
	client, ok := s.clients[host]
	if !ok {
		return nil, fmt.Errorf("unknown host: %s (available: %s)", host, strings.Join(s.hostNames, ", "))
	}
	if scope != "" && !s.hasScope(host, scope) {
		return nil, fmt.Errorf("host %s missing scope: %s", host, scope)
	}
	return client, nil
}

// hostEnum returns the enum options for the host parameter.
func (s *MultiServer) hostEnum() mcp.ToolOption {
	return mcp.WithString("host",
		mcp.Required(),
		mcp.Description(fmt.Sprintf("Target host (%s)", strings.Join(s.hostNames, "/"))),
	)
}

// --- Tool Registration (multi-instance variants) ---

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

func (s *MultiServer) addListContainersTool() {
	tool := mcp.NewTool("list_containers",
		mcp.WithDescription("List all Docker containers on a host"),
		s.hostEnum(),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		containers, err := client.ListContainers()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list containers: %v", err)), nil
		}
		data, _ := json.MarshalIndent(containers, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addContainerLogsTool() {
	tool := mcp.NewTool("container_logs",
		mcp.WithDescription("Get log output from a container on a host"),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
		mcp.WithNumber("tail", mcp.Description("Number of lines (default 100)")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		tail := 100
		if t, ok := args["tail"]; ok {
			if tf, ok := t.(float64); ok {
				tail = int(tf)
			}
		}
		logs, err := client.GetContainerLogs(id, tail)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get logs: %v", err)), nil
		}
		return mcp.NewToolResultText(logs), nil
	})
}

func (s *MultiServer) addContainerConfigTool() {
	tool := mcp.NewTool("container_config",
		mcp.WithDescription("Get the full configuration of a container"),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		config, err := client.GetContainerConfig(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get config: %v", err)), nil
		}
		return mcp.NewToolResultText(string(config)), nil
	})
}

func (s *MultiServer) addStartContainerTool() {
	tool := mcp.NewTool("start_container",
		mcp.WithDescription("Start a container on a host"),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		if err := client.StartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s started on %s", id, args["host"])), nil
	})
}

func (s *MultiServer) addStopContainerTool() {
	tool := mcp.NewTool("stop_container",
		mcp.WithDescription("Stop a container on a host"),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		if err := client.StopContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to stop: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s stopped on %s", id, args["host"])), nil
	})
}

func (s *MultiServer) addRestartContainerTool() {
	tool := mcp.NewTool("restart_container",
		mcp.WithDescription("Restart a container on a host"),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		if err := client.RestartContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to restart: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s restarted on %s", id, args["host"])), nil
	})
}

func (s *MultiServer) addCreateContainerTool() {
	tool := mcp.NewTool("create_container",
		mcp.WithDescription("Create a new container on a host"),
		s.hostEnum(),
		mcp.WithString("image", mcp.Required(), mcp.Description("Docker image")),
		mcp.WithString("name", mcp.Description("Container name")),
		mcp.WithObject("env", mcp.Description("Environment variables (key-value)")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		image, _ := args["image"].(string)
		createReq := wagmios.CreateContainerRequest{Image: image}
		if name, ok := args["name"].(string); ok {
			createReq.Name = name
		}
		if env, ok := args["env"].(map[string]interface{}); ok {
			createReq.Env = make(map[string]string)
			for k, v := range env {
				if vs, ok := v.(string); ok {
					createReq.Env[k] = vs
				}
			}
		}
		container, err := client.CreateContainer(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create: %v", err)), nil
		}
		data, _ := json.MarshalIndent(container, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addDeleteContainerTool() {
	tool := mcp.NewTool("delete_container",
		mcp.WithDescription("Delete a container on a host. Irreversible — confirm first."),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "containers:delete")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		if err := client.DeleteContainer(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Container %s deleted on %s", id, args["host"])), nil
	})
}

func (s *MultiServer) addListImagesTool() {
	tool := mcp.NewTool("list_images",
		mcp.WithDescription("List Docker images on a host"),
		s.hostEnum(),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "images:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		images, err := client.ListImages()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list images: %v", err)), nil
		}
		data, _ := json.MarshalIndent(images, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addPullImageTool() {
	tool := mcp.NewTool("pull_image",
		mcp.WithDescription("Pull a Docker image on a host"),
		s.hostEnum(),
		mcp.WithString("image", mcp.Required(), mcp.Description("Image name")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "images:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		image, _ := args["image"].(string)
		if err := client.PullImage(image); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to pull: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s pulled on %s", image, args["host"])), nil
	})
}

func (s *MultiServer) addDeleteImageTool() {
	tool := mcp.NewTool("delete_image",
		mcp.WithDescription("Delete a Docker image on a host. Irreversible — confirm first."),
		s.hostEnum(),
		mcp.WithString("id", mcp.Required(), mcp.Description("Image ID")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "images:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, _ := args["id"].(string)
		if err := client.DeleteImage(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete image: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Image %s deleted on %s", id, args["host"])), nil
	})
}

func (s *MultiServer) addBrowseMarketplaceTool() {
	tool := mcp.NewTool("browse_marketplace",
		mcp.WithDescription("Browse available marketplace apps on a host"),
		s.hostEnum(),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		apps, err := client.BrowseMarketplace()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to browse: %v", err)), nil
		}
		data, _ := json.MarshalIndent(apps, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addGetMarketplaceAppTool() {
	tool := mcp.NewTool("get_marketplace_app",
		mcp.WithDescription("Get details for a marketplace app on a host"),
		s.hostEnum(),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		appID, _ := args["app_id"].(string)
		app, err := client.GetMarketplaceApp(appID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get app: %v", err)), nil
		}
		data, _ := json.MarshalIndent(app, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addListInstalledAppsTool() {
	tool := mcp.NewTool("list_installed_apps",
		mcp.WithDescription("List installed marketplace apps on a host"),
		s.hostEnum(),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "marketplace:read")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		apps, err := client.ListInstalledApps()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list apps: %v", err)), nil
		}
		data, _ := json.MarshalIndent(apps, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addInstallMarketplaceAppTool() {
	tool := mcp.NewTool("install_app",
		mcp.WithDescription("Install a marketplace app on a host (does not start it)"),
		s.hostEnum(),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID")),
		mcp.WithString("container_name", mcp.Description("Container name")),
		mcp.WithString("custom_name", mcp.Description("Custom app name")),
		mcp.WithObject("environment", mcp.Description("Environment variables (key-value)")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "marketplace:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		appID, _ := args["app_id"].(string)
		createReq := wagmios.MarketplaceCreateRequest{AppID: appID}
		if name, ok := args["container_name"].(string); ok {
			createReq.ContainerName = name
		}
		if name, ok := args["custom_name"].(string); ok {
			createReq.CustomName = name
		}
		if env, ok := args["environment"].(map[string]interface{}); ok {
			createReq.Environment = make(map[string]string)
			for k, v := range env {
				if vs, ok := v.(string); ok {
					createReq.Environment[k] = vs
				}
			}
		}
		result, err := client.CreateMarketplaceApp(createReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to install: %v", err)), nil
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func (s *MultiServer) addStartMarketplaceAppTool() {
	tool := mcp.NewTool("start_app",
		mcp.WithDescription("Start an installed marketplace app on a host"),
		s.hostEnum(),
		mcp.WithString("app_id", mcp.Required(), mcp.Description("App ID")),
		mcp.WithString("container_name", mcp.Required(), mcp.Description("Container name")),
		mcp.WithString("compose_path", mcp.Required(), mcp.Description("Compose file path from install_app")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]interface{})
		client, err := s.getClient(args, "marketplace:write")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startReq := wagmios.MarketplaceStartRequest{
			AppID:         args["app_id"].(string),
			ContainerName: args["container_name"].(string),
			ComposePath:   args["compose_path"].(string),
		}
		if err := client.StartMarketplaceApp(startReq); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("App %s started on %s", startReq.AppID, args["host"])), nil
	})
}
package wagmios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// Client is a WAGMIOS REST API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new WAGMIOS API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIResponse is the standard WAGMIOS API response wrapper.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *APIError       `json:"error,omitempty"`
}

// APIError is a WAGMIOS API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// AuthStatus represents the key status response.
type AuthStatus struct {
	HasKey         bool     `json:"has_key"`
	WizardRequired bool     `json:"wizard_required"`
	Meta           KeyMeta  `json:"meta"`
}

// KeyMeta holds API key metadata.
type KeyMeta struct {
	ID        string   `json:"id"`
	KeyPrefix string   `json:"key_prefix"`
	Label     string   `json:"label"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
	LastUsedAt string  `json:"last_used_at,omitempty"`
}

// Container represents a Docker container.
type Container struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	State   string   `json:"state"`
	Status  string   `json:"status"`
	Created string   `json:"created"`
	Ports   []Port   `json:"ports,omitempty"`
}

// Port represents a container port mapping.
type Port struct {
	Private  int    `json:"private"`
	Public   int    `json:"public,omitempty"`
	Protocol string `json:"protocol"`
}

// Image represents a Docker image.
type Image struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       int64  `json:"size"`
	Created    string `json:"created"`
}

// MarketplaceApp represents a marketplace app.
type MarketplaceApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Port        int    `json:"port"`
	Category    string `json:"category"`
}

// MarketplaceInstalledApp represents an installed marketplace app.
type MarketplaceInstalledApp struct {
	AppID         string `json:"app_id"`
	AppName       string `json:"app_name"`
	ContainerName string `json:"container_name"`
	Status        string `json:"status"`
}

// CreateContainerRequest is the body for creating a container.
type CreateContainerRequest struct {
	Image   string            `json:"image"`
	Name    string            `json:"name"`
	Env     map[string]string `json:"env,omitempty"`
	Ports   []Port            `json:"ports,omitempty"`
	Volumes []Volume          `json:"volumes,omitempty"`
}

// Volume represents a container volume mount.
type Volume struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

// MarketplaceCreateRequest is the body for installing a marketplace app.
type MarketplaceCreateRequest struct {
	AppID         string            `json:"app_id"`
	ContainerName string            `json:"container_name,omitempty"`
	CustomName    string            `json:"custom_name,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
}

// MarketplaceCreateResponse is the response from installing a marketplace app.
type MarketplaceCreateResponse struct {
	AppID      string `json:"app_id"`
	AppName    string `json:"app_name"`
	ComposePath string `json:"compose_path"`
	InstallDir  string `json:"install_dir"`
	Status     string `json:"status"`
}

// MarketplaceStartRequest is the body for starting a marketplace app.
type MarketplaceStartRequest struct {
	AppID         string `json:"app_id"`
	ContainerName string `json:"container_name"`
	ComposePath   string `json:"compose_path"`
}

// SystemInfo represents system information.
type SystemInfo struct {
	DockerVersion string `json:"docker_version"`
	APIVersion    string `json:"api_version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
}

// SystemMetrics represents system resource metrics.
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPercent   float64 `json:"disk_percent"`
	Containers    int     `json:"containers"`
	Images        int     `json:"images"`
}

// doRequest performs an authenticated HTTP request against the WAGMIOS API.
func (c *Client) doRequest(method, path string, body interface{}) (*APIResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status before attempting JSON decode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		// Drain remaining body for connection reuse
		if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
			log.Printf("warn: failed to drain response body: %v", drainErr)
		}
		if readErr != nil {
			return nil, fmt.Errorf("HTTP %d: failed to read error body: %w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.Success && apiResp.Error != nil {
		return &apiResp, apiResp.Error
	}

	return &apiResp, nil
}

// Get sends a GET request.
func (c *Client) Get(path string) (*APIResponse, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

// Post sends a POST request.
func (c *Client) Post(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest(http.MethodPost, path, body)
}

// Delete sends a DELETE request.
func (c *Client) Delete(path string) (*APIResponse, error) {
	return c.doRequest(http.MethodDelete, path, nil)
}

// GetAuthStatus checks the current key's status and scopes.
func (c *Client) GetAuthStatus() (*AuthStatus, error) {
	resp, err := c.Get("/api/auth/status")
	if err != nil {
		return nil, err
	}
	var status AuthStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal auth status: %w", err)
	}
	return &status, nil
}

// ListContainers returns all containers.
func (c *Client) ListContainers() ([]Container, error) {
	resp, err := c.Get("/api/containers")
	if err != nil {
		return nil, err
	}
	var containers []Container
	if err := json.Unmarshal(resp.Data, &containers); err != nil {
		return nil, fmt.Errorf("unmarshal containers: %w", err)
	}
	return containers, nil
}

// GetContainerLogs returns container log output.
func (c *Client) GetContainerLogs(id string, tail int) (string, error) {
	path := fmt.Sprintf("/api/containers/%s/logs?tail=%d", url.PathEscape(id), tail)
	resp, err := c.Get(path)
	if err != nil {
		return "", err
	}
	var logs string
	if err := json.Unmarshal(resp.Data, &logs); err != nil {
		return "", fmt.Errorf("unmarshal logs: %w", err)
	}
	return logs, nil
}

// GetContainerConfig returns the full container configuration.
func (c *Client) GetContainerConfig(id string) (json.RawMessage, error) {
	resp, err := c.Get(fmt.Sprintf("/api/containers/%s/config", url.PathEscape(id)))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// StartContainer starts a container.
func (c *Client) StartContainer(id string) error {
	_, err := c.Post(fmt.Sprintf("/api/containers/%s/start", url.PathEscape(id)), nil)
	return err
}

// StopContainer stops a container.
func (c *Client) StopContainer(id string) error {
	_, err := c.Post(fmt.Sprintf("/api/containers/%s/stop", url.PathEscape(id)), nil)
	return err
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(id string) error {
	_, err := c.Post(fmt.Sprintf("/api/containers/%s/restart", url.PathEscape(id)), nil)
	return err
}

// DeleteContainer deletes a container.
func (c *Client) DeleteContainer(id string) error {
	_, err := c.Delete(fmt.Sprintf("/api/containers/%s/delete", url.PathEscape(id)))
	return err
}

// CreateContainer creates a new container.
func (c *Client) CreateContainer(req CreateContainerRequest) (*Container, error) {
	resp, err := c.Post("/api/containers", req)
	if err != nil {
		return nil, err
	}
	var container Container
	if err := json.Unmarshal(resp.Data, &container); err != nil {
		return nil, fmt.Errorf("unmarshal container: %w", err)
	}
	return &container, nil
}

// ListImages returns all Docker images.
func (c *Client) ListImages() ([]Image, error) {
	resp, err := c.Get("/api/images")
	if err != nil {
		return nil, err
	}
	var images []Image
	if err := json.Unmarshal(resp.Data, &images); err != nil {
		return nil, fmt.Errorf("unmarshal images: %w", err)
	}
	return images, nil
}

// PullImage pulls a Docker image.
func (c *Client) PullImage(image string) error {
	_, err := c.Post("/api/images/pull", map[string]string{"image": image})
	return err
}

// DeleteImage deletes a Docker image.
func (c *Client) DeleteImage(id string) error {
	_, err := c.Delete(fmt.Sprintf("/api/images/%s", url.PathEscape(id)))
	return err
}

// BrowseMarketplace returns all available marketplace apps.
func (c *Client) BrowseMarketplace() ([]MarketplaceApp, error) {
	resp, err := c.Get("/api/marketplace")
	if err != nil {
		return nil, err
	}
	var apps []MarketplaceApp
	if err := json.Unmarshal(resp.Data, &apps); err != nil {
		return nil, fmt.Errorf("unmarshal marketplace apps: %w", err)
	}
	return apps, nil
}

// GetMarketplaceApp returns details for a specific app.
func (c *Client) GetMarketplaceApp(appID string) (*MarketplaceApp, error) {
	resp, err := c.Get(fmt.Sprintf("/api/marketplace/%s", url.PathEscape(appID)))
	if err != nil {
		return nil, err
	}
	var app MarketplaceApp
	if err := json.Unmarshal(resp.Data, &app); err != nil {
		return nil, fmt.Errorf("unmarshal marketplace app: %w", err)
	}
	return &app, nil
}

// ListInstalledApps returns all installed marketplace apps.
func (c *Client) ListInstalledApps() ([]MarketplaceInstalledApp, error) {
	resp, err := c.Get("/api/marketplace/installed")
	if err != nil {
		return nil, err
	}
	var apps []MarketplaceInstalledApp
	if err := json.Unmarshal(resp.Data, &apps); err != nil {
		return nil, fmt.Errorf("unmarshal installed apps: %w", err)
	}
	return apps, nil
}

// CreateMarketplaceApp downloads a marketplace app compose file.
func (c *Client) CreateMarketplaceApp(req MarketplaceCreateRequest) (*MarketplaceCreateResponse, error) {
	resp, err := c.Post("/api/marketplace/create", req)
	if err != nil {
		return nil, err
	}
	var result MarketplaceCreateResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal marketplace create response: %w", err)
	}
	return &result, nil
}

// StartMarketplaceApp starts a marketplace app.
func (c *Client) StartMarketplaceApp(req MarketplaceStartRequest) error {
	_, err := c.Post("/api/marketplace/start", req)
	return err
}

// GetSystemInfo returns Docker and system information.
func (c *Client) GetSystemInfo() (*SystemInfo, error) {
	resp, err := c.Get("/api/system/info")
	if err != nil {
		return nil, err
	}
	var info SystemInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal system info: %w", err)
	}
	return &info, nil
}

// GetSystemMetrics returns system resource metrics.
func (c *Client) GetSystemMetrics() (*SystemMetrics, error) {
	resp, err := c.Get("/api/system/metrics")
	if err != nil {
		return nil, err
	}
	var metrics SystemMetrics
	if err := json.Unmarshal(resp.Data, &metrics); err != nil {
		return nil, fmt.Errorf("unmarshal system metrics: %w", err)
	}
	return &metrics, nil
}
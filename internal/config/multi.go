package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// MultiConfig holds configuration for multiple WAGMIOS instances.
// Each instance has its own URL and API key, allowing one ClawMachine
// server to manage multiple hosts.
type MultiConfig struct {
	Instances map[string]InstanceConfig `json:"instances"`
}

// InstanceConfig holds the connection details for a single WAGMIOS instance.
type InstanceConfig struct {
	URL   string `json:"url"`   // WAGMIOS backend URL (e.g. http://192.168.1.10:5179)
	Key   string `json:"key"`   // WAGMIOS API key
	Label string `json:"label"` // Human-readable label (e.g. "Homelab NAS")
}

// LoadMultiConfig reads a multi-instance config from a JSON file.
func LoadMultiConfig(path string) (*MultiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg MultiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if len(cfg.Instances) == 0 {
		return nil, fmt.Errorf("no instances defined in config")
	}
	return &cfg, nil
}
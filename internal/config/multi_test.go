package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMultiConfig_valid(t *testing.T) {
	content := `{
		"instances": {
			"nas": {
				"url": "http://192.168.1.10:5179",
				"key": "wag_live_test123",
				"label": "Homelab NAS"
			}
		}
	}`

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMultiConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiConfig returned error: %v", err)
	}
	if len(cfg.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(cfg.Instances))
	}
	inst, ok := cfg.Instances["nas"]
	if !ok {
		t.Fatal("expected 'nas' instance")
	}
	if inst.URL != "http://192.168.1.10:5179" {
		t.Errorf("expected URL http://192.168.1.10:5179, got %s", inst.URL)
	}
	if inst.Key != "wag_live_test123" {
		t.Errorf("expected key wag_live_test123, got %s", inst.Key)
	}
	if inst.Label != "Homelab NAS" {
		t.Errorf("expected label 'Homelab NAS', got %s", inst.Label)
	}
}

func TestLoadMultiConfig_empty(t *testing.T) {
	content := `{"instances": {}}`

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMultiConfig(path)
	if err == nil {
		t.Error("expected error for empty instances")
	}
}

func TestLoadMultiConfig_missingFile(t *testing.T) {
	_, err := LoadMultiConfig("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadMultiConfig_invalidJSON(t *testing.T) {
	content := `{not valid json}`

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMultiConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadMultiConfig_multipleInstances(t *testing.T) {
	content := `{
		"instances": {
			"nas": {
				"url": "http://192.168.1.10:5179",
				"key": "wag_live_aaa",
				"label": "NAS"
			},
			"vps": {
				"url": "http://192.168.1.20:5179",
				"key": "wag_live_bbb",
				"label": "VPS"
			}
		}
	}`

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMultiConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiConfig returned error: %v", err)
	}
	if len(cfg.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(cfg.Instances))
	}
}
package wagmios

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:5179", "wag_live_testkey")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.BaseURL != "http://localhost:5179" {
		t.Errorf("expected BaseURL http://localhost:5179, got %s", client.BaseURL)
	}
	if client.APIKey != "wag_live_testkey" {
		t.Errorf("expected APIKey wag_live_testkey, got %s", client.APIKey)
	}
}

func TestClient_timeout(t *testing.T) {
	client := NewClient("http://localhost:5179", "test")
	if client.HTTPClient == nil {
		t.Fatal("HTTPClient should be initialized")
	}
	if client.HTTPClient.Timeout.Seconds() != 30 {
		t.Errorf("expected 30s timeout, got %v", client.HTTPClient.Timeout)
	}
}
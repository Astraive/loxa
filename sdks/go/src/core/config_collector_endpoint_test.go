package core

import "testing"

func TestNewClient_AcceptsCollectorEndpointAlias(t *testing.T) {
	client, err := NewClient(Config{
		CollectorEndpoint: "http://localhost:9308",
		Service:           "checkout",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	if client.cfg.CollectorURL != "http://localhost:9308" {
		t.Fatalf("CollectorURL = %q, want alias value", client.cfg.CollectorURL)
	}
}

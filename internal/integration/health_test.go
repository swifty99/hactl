//go:build integration

package integration

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/haapi"
)

func TestHealthEndToEnd(t *testing.T) {
	cfg := loadConfig(t)
	client := haapi.New(cfg.URL, cfg.Token)
	ctx := context.Background()

	// GET /api/ should succeed
	data, err := client.GetAPIStatus(ctx)
	if err != nil {
		t.Fatalf("GetAPIStatus: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GetAPIStatus returned empty body")
	}

	// GET /api/config should return version
	configData, err := client.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !bytes.Contains(configData, []byte("version")) {
		t.Error("config response missing 'version' field")
	}
	if !bytes.Contains(configData, []byte("Test Home")) {
		t.Error("config response missing location name 'Test Home'")
	}

	// GET /api/error_log should not fail
	_, err = client.GetErrorLog(ctx)
	if err != nil {
		t.Fatalf("GetErrorLog: %v", err)
	}
}

func TestHealthCommand(t *testing.T) {
	out := runHactl(t, "health")
	if !strings.Contains(out, "HA ") {
		t.Errorf("health output missing 'HA ' prefix: %s", out)
	}
	if !strings.Contains(out, "state=") {
		t.Errorf("health output missing 'state=': %s", out)
	}
	if !strings.Contains(out, "recorder=") {
		t.Errorf("health output missing 'recorder=': %s", out)
	}
	if !strings.Contains(out, "Test Home") {
		t.Errorf("health output missing location name: %s", out)
	}
}

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(ha.Dir())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

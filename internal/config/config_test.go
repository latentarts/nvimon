package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.RefreshInterval != DefaultRefreshInterval {
		t.Fatalf("refresh interval = %v, want %v", cfg.RefreshInterval, DefaultRefreshInterval)
	}

	if cfg.HistoryLength != DefaultHistoryLength {
		t.Fatalf("history length = %d, want %d", cfg.HistoryLength, DefaultHistoryLength)
	}

	if cfg.Agent.BindAddress != DefaultAgentBind {
		t.Fatalf("bind address = %q, want %q", cfg.Agent.BindAddress, DefaultAgentBind)
	}

	if len(cfg.Hosts) != 1 || cfg.Hosts[0].Mode != HostModeLocal {
		t.Fatalf("default hosts = %+v, want one local host", cfg.Hosts)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("hosts:\n  - name: gpu-a\n    mode: remote\n    url: http://gpu-a:9910\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.RefreshInterval != DefaultRefreshInterval {
		t.Fatalf("refresh interval = %v, want %v", cfg.RefreshInterval, DefaultRefreshInterval)
	}
	if cfg.Agent.BindAddress != DefaultAgentBind {
		t.Fatalf("bind address = %q, want %q", cfg.Agent.BindAddress, DefaultAgentBind)
	}
	if len(cfg.Hosts) != 1 || cfg.Hosts[0].URL != "http://gpu-a:9910" {
		t.Fatalf("hosts = %+v", cfg.Hosts)
	}
}

func TestLoadConfigRejectsInvalidRemoteHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("hosts:\n  - name: gpu-a\n    mode: remote\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected load error for missing remote url")
	}
}

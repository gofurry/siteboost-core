package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
	if cfg.Mode != ModeProxyOnly {
		t.Fatalf("mode = %q, want %q", cfg.Mode, ModeProxyOnly)
	}
	if cfg.Proxy.NonSteamBehavior != NonSteamReject {
		t.Fatalf("non-steam behavior = %q, want %q", cfg.Proxy.NonSteamBehavior, NonSteamReject)
	}
}

func TestLoadFileYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
mode: proxy_only
proxy:
  listen_addr: "127.0.0.1:28080"
  non_steam_behavior: "direct"
  read_header_timeout: "3s"
rules:
  custom_domains:
    - "example.steam.test"
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Proxy.ListenAddr != "127.0.0.1:28080" {
		t.Fatalf("listen addr = %q", cfg.Proxy.ListenAddr)
	}
	if cfg.Proxy.NonSteamBehavior != NonSteamDirect {
		t.Fatalf("non-steam behavior = %q", cfg.Proxy.NonSteamBehavior)
	}
	if got := cfg.Proxy.ReadHeaderTimeout.Std(); got != 3*time.Second {
		t.Fatalf("read header timeout = %v", got)
	}
	if len(cfg.Rules.CustomDomains) != 1 {
		t.Fatalf("custom domains = %#v", cfg.Rules.CustomDomains)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "mode",
			mutate: func(cfg *Config) {
				cfg.Mode = "pac"
			},
		},
		{
			name: "non steam",
			mutate: func(cfg *Config) {
				cfg.Proxy.NonSteamBehavior = "tunnel"
			},
		},
		{
			name: "non loopback",
			mutate: func(cfg *Config) {
				cfg.Proxy.ListenAddr = "0.0.0.0:26501"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate returned nil")
			}
		})
	}
}

func TestValidateAllowsLANWhenExplicit(t *testing.T) {
	cfg := Default()
	cfg.Proxy.ListenAddr = "0.0.0.0:26501"
	cfg.Proxy.AllowLAN = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

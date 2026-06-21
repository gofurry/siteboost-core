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
	if cfg.Resolver.Mode != ResolverSystem {
		t.Fatalf("resolver mode = %q, want %q", cfg.Resolver.Mode, ResolverSystem)
	}
	if !cfg.Resolver.PreferIPv4 {
		t.Fatalf("prefer IPv4 should be enabled by default")
	}
	if cfg.Upstream.Type != UpstreamDirect {
		t.Fatalf("upstream type = %q, want %q", cfg.Upstream.Type, UpstreamDirect)
	}
	if cfg.PAC.ListenAddr != "127.0.0.1:26502" {
		t.Fatalf("pac listen addr = %q", cfg.PAC.ListenAddr)
	}
	if cfg.Runtime.RollbackPath == "" {
		t.Fatalf("rollback path is empty")
	}
	if cfg.Hosts.HTTPListenAddr != "127.0.0.1:80" || cfg.Hosts.HTTPSListenAddr != "127.0.0.1:443" {
		t.Fatalf("hosts listen addrs = %q / %q", cfg.Hosts.HTTPListenAddr, cfg.Hosts.HTTPSListenAddr)
	}
	if cfg.Cert.Dir == "" {
		t.Fatalf("cert dir is empty")
	}
}

func TestValidateFillsDefaultDoHServers(t *testing.T) {
	cfg := Default()
	cfg.Resolver.Mode = ResolverDoH
	cfg.Resolver.Servers = []string{"  "}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Resolver.Mode != ResolverDoH {
		t.Fatalf("resolver mode = %q, want %q", cfg.Resolver.Mode, ResolverDoH)
	}
	if len(cfg.Resolver.Servers) == 0 {
		t.Fatalf("default DoH servers were not filled")
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
pac:
  listen_addr: "127.0.0.1:28082"
hosts:
  map_ip: "127.0.0.1"
  http_listen_addr: "127.0.0.1:28080"
  https_listen_addr: "127.0.0.1:28443"
  path: "hosts.txt"
  extra_domains:
    - "login.steampowered.com"
cert:
  dir: "certs"
rules:
  custom_domains:
    - "example.steam.test"
resolver:
  mode: "doh"
  servers:
    - "https://dns.example/dns-query"
  cache_ttl: "1m"
  timeout: "2s"
upstream:
  type: "http"
  address: "127.0.0.1:18080"
  username: "user"
  password: "secret"
  enable_default_steam_profiles: false
  profiles:
    - match_domains:
        - "steamcommunity.com"
      forward_host: "steamcommunity-a.akamaihd.net"
      tls_server_name: "steamcommunity-a.akamaihd.net"
runtime:
  rollback_path: "rollback.json"
system_proxy:
  services:
    - "Wi-Fi"
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
	if cfg.Resolver.Mode != ResolverDoH {
		t.Fatalf("resolver mode = %q", cfg.Resolver.Mode)
	}
	if got := cfg.Resolver.CacheTTL.Std(); got != time.Minute {
		t.Fatalf("cache ttl = %v", got)
	}
	if cfg.Upstream.Type != UpstreamHTTP || cfg.Upstream.Address != "127.0.0.1:18080" {
		t.Fatalf("upstream = %#v", cfg.Upstream)
	}
	if cfg.Upstream.EnableDefaultSteamProfiles {
		t.Fatalf("default steam profiles should be disabled")
	}
	if len(cfg.Upstream.Profiles) != 1 {
		t.Fatalf("upstream profiles = %#v", cfg.Upstream.Profiles)
	}
	if cfg.Upstream.Profiles[0].ForwardHost != "steamcommunity-a.akamaihd.net" {
		t.Fatalf("forward host = %q", cfg.Upstream.Profiles[0].ForwardHost)
	}
	if cfg.PAC.ListenAddr != "127.0.0.1:28082" {
		t.Fatalf("pac listen addr = %q", cfg.PAC.ListenAddr)
	}
	if cfg.Hosts.HTTPSListenAddr != "127.0.0.1:28443" {
		t.Fatalf("hosts https listen addr = %q", cfg.Hosts.HTTPSListenAddr)
	}
	if len(cfg.Hosts.ExtraDomains) != 1 || cfg.Hosts.ExtraDomains[0] != "login.steampowered.com" {
		t.Fatalf("hosts extra domains = %#v", cfg.Hosts.ExtraDomains)
	}
	if cfg.Cert.Dir != "certs" {
		t.Fatalf("cert dir = %q", cfg.Cert.Dir)
	}
	if cfg.Runtime.RollbackPath != "rollback.json" {
		t.Fatalf("rollback path = %q", cfg.Runtime.RollbackPath)
	}
	if len(cfg.System.Services) != 1 || cfg.System.Services[0] != "Wi-Fi" {
		t.Fatalf("services = %#v", cfg.System.Services)
	}
}

func TestValidateAllowsKnownModes(t *testing.T) {
	for _, mode := range []string{ModePAC, ModeSystem, ModeHosts, "proxy-only"} {
		t.Run(mode, func(t *testing.T) {
			cfg := Default()
			cfg.Mode = mode
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
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
				cfg.Mode = "bogus"
			},
		},
		{
			name: "pac listen addr",
			mutate: func(cfg *Config) {
				cfg.PAC.ListenAddr = "0.0.0.0:26502"
			},
		},
		{
			name: "rollback path",
			mutate: func(cfg *Config) {
				cfg.Runtime.RollbackPath = ""
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
		{
			name: "resolver mode",
			mutate: func(cfg *Config) {
				cfg.Resolver.Mode = "bogus"
			},
		},
		{
			name: "resolver servers",
			mutate: func(cfg *Config) {
				cfg.Resolver.Mode = ResolverUDP
				cfg.Resolver.Servers = nil
			},
		},
		{
			name: "ip preference conflict",
			mutate: func(cfg *Config) {
				cfg.Resolver.PreferIPv4 = true
				cfg.Resolver.PreferIPv6 = true
			},
		},
		{
			name: "upstream type",
			mutate: func(cfg *Config) {
				cfg.Upstream.Type = "wireguard"
			},
		},
		{
			name: "upstream address",
			mutate: func(cfg *Config) {
				cfg.Upstream.Type = UpstreamSOCKS5
			},
		},
		{
			name: "upstream profile domain",
			mutate: func(cfg *Config) {
				cfg.Upstream.Profiles = []OutboundProfileConfig{{
					MatchDomains: []string{"foo.*.example"},
					ForwardHost:  "cdn-a.akamaihd.net",
				}}
			},
		},
		{
			name: "upstream profile ip",
			mutate: func(cfg *Config) {
				cfg.Upstream.Profiles = []OutboundProfileConfig{{
					MatchDomains: []string{"steamcommunity.com"},
					CandidateIPs: []string{"not-an-ip"},
				}}
			},
		},
		{
			name: "hosts listen addr",
			mutate: func(cfg *Config) {
				cfg.Hosts.HTTPListenAddr = "0.0.0.0:80"
			},
		},
		{
			name: "hosts wildcard extra",
			mutate: func(cfg *Config) {
				cfg.Hosts.ExtraDomains = []string{"*.example.com"}
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

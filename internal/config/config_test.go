package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if len(cfg.Providers.Enabled) != 1 || cfg.Providers.Enabled[0] != "steam" {
		t.Fatalf("providers = %#v", cfg.Providers.Enabled)
	}
	if cfg.Proxy.NonTargetBehavior != NonTargetReject {
		t.Fatalf("non-target behavior = %q, want %q", cfg.Proxy.NonTargetBehavior, NonTargetReject)
	}
	if cfg.DNS.Enabled {
		t.Fatalf("dns intercept should be disabled by default")
	}
	if cfg.DNS.Strategy != DNSInterceptManual {
		t.Fatalf("dns intercept strategy = %q, want %q", cfg.DNS.Strategy, DNSInterceptManual)
	}
	if cfg.DNS.ListenAddr != "127.0.0.1:53" {
		t.Fatalf("dns listen addr = %q", cfg.DNS.ListenAddr)
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
	if !cfg.Cert.AutoInstall {
		t.Fatalf("cert auto install should be enabled by default")
	}
	if cfg.Cert.StoreScope != CertStoreMachine {
		t.Fatalf("cert store scope = %q, want %q", cfg.Cert.StoreScope, CertStoreMachine)
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

func TestValidateAcceptsExplicitSystemDNS(t *testing.T) {
	cfg := Default()
	cfg.Mode = ModeDNS
	cfg.DNS.Strategy = DNSInterceptSystem
	cfg.DNS.ListenAddr = "127.0.0.1:53"
	cfg.DNS.Interfaces = []string{" Wi-Fi ", "Wi-Fi", "Ethernet"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(cfg.DNS.Interfaces) != 2 || cfg.DNS.Interfaces[0] != "Wi-Fi" || cfg.DNS.Interfaces[1] != "Ethernet" {
		t.Fatalf("interfaces = %#v", cfg.DNS.Interfaces)
	}
}

func TestValidateAcceptsPageEnhanceConfig(t *testing.T) {
	cfg := Default()
	cfg.PageEnhance.Enabled = true
	cfg.PageEnhance.OnError = "fail-closed"
	cfg.PageEnhance.MaxBodySize = 4096
	cfg.PageEnhance.Assets = []PageEnhanceAssetConfig{{
		Path:        "/local.js",
		File:        "assets/local.js",
		ContentType: "application/javascript",
	}}
	cfg.PageEnhance.Transforms = []PageEnhanceTransformConfig{{
		Name: "demo",
		Match: PageEnhanceMatchConfig{
			Providers:    []string{" Steam "},
			Hosts:        []string{"*.SteamCommunity.com"},
			PathPrefixes: []string{"/"},
			ContentTypes: []string{"Text/HTML"},
			StatusCodes:  []int{200},
		},
		Headers: PageEnhanceHeadersConfig{
			Set:    map[string]string{"x-enhanced": "yes"},
			Remove: []string{"etag"},
		},
		InjectBody: `<script src="/local.js"></script>`,
		Replace: []PageEnhanceReplaceConfig{{
			Old: "old",
			New: "new",
		}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.PageEnhance.OnError != PageEnhanceFailClosed {
		t.Fatalf("on_error = %q", cfg.PageEnhance.OnError)
	}
	if got := cfg.PageEnhance.Transforms[0].Match.Hosts[0]; got != "*.steamcommunity.com" {
		t.Fatalf("host = %q", got)
	}
	if got := cfg.PageEnhance.Transforms[0].Match.Providers[0]; got != "steam" {
		t.Fatalf("provider = %q", got)
	}
	if _, ok := cfg.PageEnhance.Transforms[0].Headers.Set["X-Enhanced"]; !ok {
		t.Fatalf("headers = %#v", cfg.PageEnhance.Transforms[0].Headers.Set)
	}
}

func TestLoadFileYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
mode: proxy_only
proxy:
  listen_addr: "127.0.0.1:28080"
  non_target_behavior: "direct"
  read_header_timeout: "3s"
providers:
  enabled:
    - steam
    - github
dns_intercept:
  strategy: "manual"
  listen_addr: "127.0.0.1:15353"
  map_ipv4: "127.0.0.2"
  map_ipv6: "::1"
  ttl: "15s"
  block_https_records: true
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
  auto_install: false
  store_scope: "current_user"
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
	if cfg.Proxy.NonTargetBehavior != NonTargetDirect {
		t.Fatalf("non-target behavior = %q", cfg.Proxy.NonTargetBehavior)
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
	if len(cfg.Providers.Enabled) != 2 || cfg.Providers.Enabled[0] != "steam" || cfg.Providers.Enabled[1] != "github" {
		t.Fatalf("providers = %#v", cfg.Providers.Enabled)
	}
	if cfg.DNS.Enabled {
		t.Fatalf("dns intercept should not be enabled outside dns mode")
	}
	if cfg.DNS.ListenAddr != "127.0.0.1:15353" {
		t.Fatalf("dns listen addr = %q", cfg.DNS.ListenAddr)
	}
	if cfg.DNS.MapIPv4 != "127.0.0.2" || cfg.DNS.MapIPv6 != "::1" {
		t.Fatalf("dns map addrs = %q / %q", cfg.DNS.MapIPv4, cfg.DNS.MapIPv6)
	}
	if got := cfg.DNS.TTL.Std(); got != 15*time.Second {
		t.Fatalf("dns ttl = %v", got)
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
	if cfg.Cert.AutoInstall {
		t.Fatalf("cert auto install should be disabled")
	}
	if cfg.Cert.StoreScope != CertStoreUser {
		t.Fatalf("cert store scope = %q, want %q", cfg.Cert.StoreScope, CertStoreUser)
	}
	if cfg.Runtime.RollbackPath != "rollback.json" {
		t.Fatalf("rollback path = %q", cfg.Runtime.RollbackPath)
	}
	if len(cfg.System.Services) != 1 || cfg.System.Services[0] != "Wi-Fi" {
		t.Fatalf("services = %#v", cfg.System.Services)
	}
}

func TestValidateAllowsKnownModes(t *testing.T) {
	for _, mode := range []string{ModePAC, ModeSystem, ModeHosts, ModeDNS, "proxy-only", "dns-intercept"} {
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
			name: "non target",
			mutate: func(cfg *Config) {
				cfg.Proxy.NonTargetBehavior = "tunnel"
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
		{
			name: "dns enabled outside dns mode",
			mutate: func(cfg *Config) {
				cfg.DNS.Enabled = true
			},
		},
		{
			name: "dns system strategy requires interfaces",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.Strategy = DNSInterceptSystem
			},
		},
		{
			name: "dns system strategy requires port 53",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.Strategy = DNSInterceptSystem
				cfg.DNS.ListenAddr = "127.0.0.1:15353"
				cfg.DNS.Interfaces = []string{"Wi-Fi"}
			},
		},
		{
			name: "dns system strategy requires loopback",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.Strategy = DNSInterceptSystem
				cfg.DNS.ListenAddr = "192.0.2.10:53"
				cfg.DNS.Interfaces = []string{"Wi-Fi"}
			},
		},
		{
			name: "dns external strategy not implemented",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.Strategy = DNSInterceptExternal
			},
		},
		{
			name: "dns listen addr",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.ListenAddr = "0.0.0.0:53"
			},
		},
		{
			name: "dns map ip",
			mutate: func(cfg *Config) {
				cfg.Mode = ModeDNS
				cfg.DNS.MapIPv4 = "not-an-ip"
			},
		},
		{
			name: "page enhance asset path",
			mutate: func(cfg *Config) {
				cfg.PageEnhance.Enabled = true
				cfg.PageEnhance.Assets = []PageEnhanceAssetConfig{{Path: "asset.js", File: "asset.js"}}
			},
		},
		{
			name: "page enhance header injection",
			mutate: func(cfg *Config) {
				cfg.PageEnhance.Enabled = true
				cfg.PageEnhance.Transforms = []PageEnhanceTransformConfig{{
					Headers: PageEnhanceHeadersConfig{Set: map[string]string{"X-Test": "bad\r\nvalue"}},
				}}
			},
		},
		{
			name: "page enhance replace old",
			mutate: func(cfg *Config) {
				cfg.PageEnhance.Enabled = true
				cfg.PageEnhance.Transforms = []PageEnhanceTransformConfig{{
					Replace: []PageEnhanceReplaceConfig{{New: "new"}},
				}}
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

func TestLoadFileRejectsLegacyConfigKeys(t *testing.T) {
	tests := map[string]string{
		"proxy.non_steam_behavior": `
proxy:
  non_steam_behavior: direct
`,
		"rules.enable_default_steam_rules": `
rules:
  enable_default_steam_rules: false
`,
		"upstream.enable_default_steam_profiles": `
upstream:
  enable_default_steam_profiles: false
`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadFile(path)
			if err == nil {
				t.Fatalf("expected legacy key error")
			}
			if !strings.Contains(err.Error(), name) && !strings.Contains(err.Error(), "removed in v0.7") {
				t.Fatalf("error = %v", err)
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

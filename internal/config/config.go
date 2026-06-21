package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/rules"
	"gopkg.in/yaml.v3"
)

const (
	ModeProxyOnly = "proxy_only"
	ModePAC       = "pac"
	ModeSystem    = "system"
	ModeHosts     = "hosts"

	NonSteamReject = "reject"
	NonSteamDirect = "direct"

	ResolverSystem = "system"
	ResolverUDP    = "udp"
	ResolverTCP    = "tcp"
	ResolverDoH    = "doh"

	UpstreamDirect = "direct"
	UpstreamHTTP   = "http"
	UpstreamSOCKS5 = "socks5"
)

func DefaultDoHServers() []string {
	return []string{
		"https://dns.alidns.com/dns-query",
		"https://doh.pub/dns-query",
		"https://cloudflare-dns.com/dns-query",
		"https://dns.google/dns-query",
	}
}

type Duration time.Duration

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) MarshalYAML() (any, error) {
	return d.Std().String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar")
	}
	if strings.TrimSpace(value.Value) == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	*d = Duration(parsed)
	return nil
}

type Config struct {
	Mode     string         `yaml:"mode"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	PAC      PACConfig      `yaml:"pac"`
	Hosts    HostsConfig    `yaml:"hosts"`
	Cert     CertConfig     `yaml:"cert"`
	Resolver ResolverConfig `yaml:"resolver"`
	Upstream UpstreamConfig `yaml:"upstream"`
	System   SystemConfig   `yaml:"system_proxy"`
	Rules    RulesConfig    `yaml:"rules"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
}

type ProxyConfig struct {
	ListenAddr        string   `yaml:"listen_addr"`
	NonSteamBehavior  string   `yaml:"non_steam_behavior"`
	AllowLAN          bool     `yaml:"allow_lan"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	DialTimeout       Duration `yaml:"dial_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

type RulesConfig struct {
	EnableDefaultSteamRules bool     `yaml:"enable_default_steam_rules"`
	CustomDomains           []string `yaml:"custom_domains"`
}

type PACConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	AllowLAN   bool   `yaml:"allow_lan"`
}

type HostsConfig struct {
	MapIP           string   `yaml:"map_ip"`
	HTTPListenAddr  string   `yaml:"http_listen_addr"`
	HTTPSListenAddr string   `yaml:"https_listen_addr"`
	AllowLAN        bool     `yaml:"allow_lan"`
	Path            string   `yaml:"path"`
	ExtraDomains    []string `yaml:"extra_domains"`
}

type CertConfig struct {
	Dir         string `yaml:"dir"`
	AutoInstall bool   `yaml:"auto_install"`
}

type ResolverConfig struct {
	Mode        string   `yaml:"mode"`
	Servers     []string `yaml:"servers"`
	PreferIPv4  bool     `yaml:"prefer_ipv4"`
	PreferIPv6  bool     `yaml:"prefer_ipv6"`
	DisableIPv6 bool     `yaml:"disable_ipv6"`
	CacheTTL    Duration `yaml:"cache_ttl"`
	Timeout     Duration `yaml:"timeout"`
}

type UpstreamConfig struct {
	Type                       string                  `yaml:"type"`
	Address                    string                  `yaml:"address"`
	Username                   string                  `yaml:"username"`
	Password                   string                  `yaml:"password"`
	EnableDefaultSteamProfiles bool                    `yaml:"enable_default_steam_profiles"`
	Profiles                   []OutboundProfileConfig `yaml:"profiles"`
}

type OutboundProfileConfig struct {
	MatchDomains          []string `yaml:"match_domains"`
	CandidateIPs          []string `yaml:"candidate_ips"`
	ForwardHost           string   `yaml:"forward_host"`
	TLSServerName         string   `yaml:"tls_server_name"`
	IgnoreTLSNameMismatch bool     `yaml:"ignore_tls_name_mismatch"`
}

type SystemConfig struct {
	Services []string `yaml:"services"`
}

type RuntimeConfig struct {
	StatePath    string   `yaml:"state_path"`
	RollbackPath string   `yaml:"rollback_path"`
	ControlAddr  string   `yaml:"control_addr"`
	StopTimeout  Duration `yaml:"stop_timeout"`
}

func Default() Config {
	return Config{
		Mode: ModeProxyOnly,
		Proxy: ProxyConfig{
			ListenAddr:        "127.0.0.1:26501",
			NonSteamBehavior:  NonSteamReject,
			AllowLAN:          false,
			ReadHeaderTimeout: Duration(10 * time.Second),
			IdleTimeout:       Duration(2 * time.Minute),
			DialTimeout:       Duration(30 * time.Second),
			ShutdownTimeout:   Duration(5 * time.Second),
		},
		PAC: PACConfig{
			ListenAddr: "127.0.0.1:26502",
		},
		Hosts: HostsConfig{
			MapIP:           "127.0.0.1",
			HTTPListenAddr:  "127.0.0.1:80",
			HTTPSListenAddr: "127.0.0.1:443",
			Path:            DefaultHostsPath(),
		},
		Cert: CertConfig{
			Dir:         DefaultCertDir(),
			AutoInstall: true,
		},
		Rules: RulesConfig{
			EnableDefaultSteamRules: true,
		},
		Resolver: ResolverConfig{
			Mode:       ResolverSystem,
			PreferIPv4: true,
			CacheTTL:   Duration(10 * time.Minute),
			Timeout:    Duration(5 * time.Second),
		},
		Upstream: UpstreamConfig{
			Type:                       UpstreamDirect,
			EnableDefaultSteamProfiles: true,
		},
		Runtime: RuntimeConfig{
			StatePath:    DefaultStatePath(),
			RollbackPath: DefaultRollbackPath(),
			ControlAddr:  "127.0.0.1:0",
			StopTimeout:  Duration(5 * time.Second),
		},
	}
}

func DefaultStatePath() string {
	return filepath.Join(defaultRuntimeDir(), "runtime.json")
}

func DefaultRollbackPath() string {
	return filepath.Join(defaultRuntimeDir(), "rollback.json")
}

func DefaultCertDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = defaultRuntimeDir()
	}
	return filepath.Join(base, "steam-accelerator-core", "certs")
}

func DefaultHostsPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("windir")
	}
	if root == "" {
		if filepath.Separator == '\\' {
			root = `C:\Windows`
		} else {
			return "/etc/hosts"
		}
	}
	return filepath.Join(root, "System32", "drivers", "etc", "hosts")
}

func defaultRuntimeDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "steam-accelerator-core")
}

func LoadFile(path string) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		return cfg, cfg.Validate()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	c.Mode = normalizeMode(c.Mode)
	switch c.Mode {
	case ModeProxyOnly, ModePAC, ModeSystem, ModeHosts:
	default:
		return fmt.Errorf("unsupported mode %q", c.Mode)
	}

	c.Proxy.NonSteamBehavior = normalizeNonSteamBehavior(c.Proxy.NonSteamBehavior)
	switch c.Proxy.NonSteamBehavior {
	case NonSteamReject, NonSteamDirect:
	default:
		return fmt.Errorf("unsupported non_steam_behavior %q", c.Proxy.NonSteamBehavior)
	}

	if c.Proxy.ListenAddr == "" {
		return fmt.Errorf("proxy listen_addr is required")
	}
	if err := validateListenAddr(c.Proxy.ListenAddr, c.Proxy.AllowLAN); err != nil {
		return fmt.Errorf("invalid proxy listen_addr: %w", err)
	}

	if c.PAC.ListenAddr == "" {
		return fmt.Errorf("pac listen_addr is required")
	}
	if err := validateListenAddr(c.PAC.ListenAddr, c.PAC.AllowLAN); err != nil {
		return fmt.Errorf("invalid pac listen_addr: %w", err)
	}

	if c.Hosts.MapIP == "" {
		return fmt.Errorf("hosts map_ip is required")
	}
	mapIP := net.ParseIP(c.Hosts.MapIP)
	if mapIP == nil {
		return fmt.Errorf("hosts map_ip must be an IP address")
	}
	if !c.Hosts.AllowLAN && !mapIP.IsLoopback() {
		return fmt.Errorf("hosts map_ip %q is not loopback; set hosts.allow_lan to true to map LAN addresses", c.Hosts.MapIP)
	}
	if c.Hosts.HTTPListenAddr == "" {
		return fmt.Errorf("hosts http_listen_addr is required")
	}
	if err := validateListenAddr(c.Hosts.HTTPListenAddr, c.Hosts.AllowLAN); err != nil {
		return fmt.Errorf("invalid hosts http_listen_addr: %w", err)
	}
	if c.Hosts.HTTPSListenAddr == "" {
		return fmt.Errorf("hosts https_listen_addr is required")
	}
	if err := validateListenAddr(c.Hosts.HTTPSListenAddr, c.Hosts.AllowLAN); err != nil {
		return fmt.Errorf("invalid hosts https_listen_addr: %w", err)
	}
	if strings.TrimSpace(c.Hosts.Path) == "" {
		return fmt.Errorf("hosts path is required")
	}
	c.Hosts.ExtraDomains = trimStrings(c.Hosts.ExtraDomains)
	for _, domain := range c.Hosts.ExtraDomains {
		if strings.Contains(domain, "*") {
			return fmt.Errorf("hosts extra_domains cannot contain wildcard domain %q", domain)
		}
	}
	if strings.TrimSpace(c.Cert.Dir) == "" {
		return fmt.Errorf("cert dir is required")
	}

	if c.Runtime.ControlAddr == "" {
		c.Runtime.ControlAddr = "127.0.0.1:0"
	}
	if err := validateListenAddr(c.Runtime.ControlAddr, false); err != nil {
		return fmt.Errorf("invalid runtime control_addr: %w", err)
	}

	if strings.TrimSpace(c.Runtime.StatePath) == "" {
		return fmt.Errorf("runtime state_path is required")
	}
	if strings.TrimSpace(c.Runtime.RollbackPath) == "" {
		return fmt.Errorf("runtime rollback_path is required")
	}

	c.Resolver.Mode = normalizeResolverMode(c.Resolver.Mode)
	switch c.Resolver.Mode {
	case ResolverSystem, ResolverUDP, ResolverTCP, ResolverDoH:
	default:
		return fmt.Errorf("unsupported resolver mode %q", c.Resolver.Mode)
	}
	c.Resolver.Servers = trimStrings(c.Resolver.Servers)
	if c.Resolver.Mode == ResolverDoH && len(c.Resolver.Servers) == 0 {
		c.Resolver.Servers = DefaultDoHServers()
	}
	if c.Resolver.Mode != ResolverSystem && len(c.Resolver.Servers) == 0 {
		return fmt.Errorf("resolver servers are required for mode %q", c.Resolver.Mode)
	}
	if c.Resolver.PreferIPv4 && c.Resolver.PreferIPv6 {
		return fmt.Errorf("resolver prefer_ipv4 and prefer_ipv6 cannot both be true")
	}

	c.Upstream.Type = normalizeUpstreamType(c.Upstream.Type)
	switch c.Upstream.Type {
	case UpstreamDirect, UpstreamHTTP, UpstreamSOCKS5:
	default:
		return fmt.Errorf("unsupported upstream type %q", c.Upstream.Type)
	}
	if c.Upstream.Type != UpstreamDirect && strings.TrimSpace(c.Upstream.Address) == "" {
		return fmt.Errorf("upstream address is required for type %q", c.Upstream.Type)
	}
	c.Upstream.Address = strings.TrimSpace(c.Upstream.Address)
	c.Upstream.Username = strings.TrimSpace(c.Upstream.Username)
	if err := validateOutboundProfiles(c.Upstream.Profiles); err != nil {
		return err
	}
	c.System.Services = trimStrings(c.System.Services)

	if c.Proxy.ReadHeaderTimeout.Std() <= 0 {
		return fmt.Errorf("proxy read_header_timeout must be positive")
	}
	if c.Proxy.IdleTimeout.Std() <= 0 {
		return fmt.Errorf("proxy idle_timeout must be positive")
	}
	if c.Proxy.DialTimeout.Std() <= 0 {
		return fmt.Errorf("proxy dial_timeout must be positive")
	}
	if c.Proxy.ShutdownTimeout.Std() <= 0 {
		return fmt.Errorf("proxy shutdown_timeout must be positive")
	}
	if c.Resolver.CacheTTL.Std() <= 0 {
		return fmt.Errorf("resolver cache_ttl must be positive")
	}
	if c.Resolver.Timeout.Std() <= 0 {
		return fmt.Errorf("resolver timeout must be positive")
	}
	if c.Runtime.StopTimeout.Std() <= 0 {
		return fmt.Errorf("runtime stop_timeout must be positive")
	}

	return nil
}

func validateOutboundProfiles(profiles []OutboundProfileConfig) error {
	for i := range profiles {
		profile := &profiles[i]
		profile.MatchDomains = trimStrings(profile.MatchDomains)
		if len(profile.MatchDomains) == 0 {
			return fmt.Errorf("upstream profiles[%d] match_domains is required", i)
		}
		for j, domain := range profile.MatchDomains {
			if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*.") {
				return fmt.Errorf("upstream profiles[%d] match_domains[%d] must use the *.example.com form", i, j)
			}
			if strings.HasPrefix(domain, "*.") {
				normalized, err := rules.NormalizeHost(strings.TrimPrefix(domain, "*."))
				if err != nil {
					return fmt.Errorf("upstream profiles[%d] match_domains[%d]: %w", i, j, err)
				}
				profile.MatchDomains[j] = "*." + normalized
				continue
			}
			normalized, err := rules.NormalizeHost(domain)
			if err != nil {
				return fmt.Errorf("upstream profiles[%d] match_domains[%d]: %w", i, j, err)
			}
			profile.MatchDomains[j] = normalized
		}

		profile.CandidateIPs = trimStrings(profile.CandidateIPs)
		for j, ip := range profile.CandidateIPs {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("upstream profiles[%d] candidate_ips[%d] must be an IP address", i, j)
			}
		}
		var err error
		profile.ForwardHost, err = normalizeOptionalProfileHost(profile.ForwardHost)
		if err != nil {
			return fmt.Errorf("upstream profiles[%d] forward_host: %w", i, err)
		}
		profile.TLSServerName, err = normalizeOptionalProfileHost(profile.TLSServerName)
		if err != nil {
			return fmt.Errorf("upstream profiles[%d] tls_server_name: %w", i, err)
		}
		if len(profile.CandidateIPs) == 0 &&
			profile.ForwardHost == "" &&
			profile.TLSServerName == "" &&
			!profile.IgnoreTLSNameMismatch {
			return fmt.Errorf("upstream profiles[%d] must define candidate_ips, forward_host, tls_server_name, or ignore_tls_name_mismatch", i)
		}
	}
	return nil
}

func normalizeOptionalProfileHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	return rules.NormalizeHost(host)
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeProxyOnly, "proxy-only":
		return ModeProxyOnly
	case ModePAC:
		return ModePAC
	case ModeSystem:
		return ModeSystem
	case ModeHosts:
		return ModeHosts
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func normalizeNonSteamBehavior(behavior string) string {
	switch strings.ToLower(strings.TrimSpace(behavior)) {
	case "", NonSteamReject:
		return NonSteamReject
	case NonSteamDirect:
		return NonSteamDirect
	default:
		return strings.ToLower(strings.TrimSpace(behavior))
	}
}

func normalizeResolverMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ResolverSystem:
		return ResolverSystem
	case ResolverUDP:
		return ResolverUDP
	case ResolverTCP:
		return ResolverTCP
	case ResolverDoH:
		return ResolverDoH
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func normalizeUpstreamType(upstreamType string) string {
	switch strings.ToLower(strings.TrimSpace(upstreamType)) {
	case "", UpstreamDirect:
		return UpstreamDirect
	case UpstreamHTTP:
		return UpstreamHTTP
	case UpstreamSOCKS5:
		return UpstreamSOCKS5
	default:
		return strings.ToLower(strings.TrimSpace(upstreamType))
	}
}

func trimStrings(values []string) []string {
	trimmed := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func validateListenAddr(addr string, allowLAN bool) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if strings.TrimSpace(port) == "" {
		return fmt.Errorf("port is required")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("host must be an IP address or localhost")
	}
	if !allowLAN && !ip.IsLoopback() {
		return fmt.Errorf("host %q is not loopback; set allow_lan to true to listen on LAN addresses", host)
	}
	return nil
}

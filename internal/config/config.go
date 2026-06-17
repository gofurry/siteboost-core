package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ModeProxyOnly = "proxy_only"

	NonSteamReject = "reject"
	NonSteamDirect = "direct"
)

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
	Mode    string        `yaml:"mode"`
	Proxy   ProxyConfig   `yaml:"proxy"`
	Rules   RulesConfig   `yaml:"rules"`
	Runtime RuntimeConfig `yaml:"runtime"`
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

type RuntimeConfig struct {
	StatePath   string   `yaml:"state_path"`
	ControlAddr string   `yaml:"control_addr"`
	StopTimeout Duration `yaml:"stop_timeout"`
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
		Rules: RulesConfig{
			EnableDefaultSteamRules: true,
		},
		Runtime: RuntimeConfig{
			StatePath:   DefaultStatePath(),
			ControlAddr: "127.0.0.1:0",
			StopTimeout: Duration(5 * time.Second),
		},
	}
}

func DefaultStatePath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "steam-accelerator-core", "runtime.json")
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
	if c.Mode != ModeProxyOnly {
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

	if c.Runtime.ControlAddr == "" {
		c.Runtime.ControlAddr = "127.0.0.1:0"
	}
	if err := validateListenAddr(c.Runtime.ControlAddr, false); err != nil {
		return fmt.Errorf("invalid runtime control_addr: %w", err)
	}

	if strings.TrimSpace(c.Runtime.StatePath) == "" {
		return fmt.Errorf("runtime state_path is required")
	}

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
	if c.Runtime.StopTimeout.Std() <= 0 {
		return fmt.Errorf("runtime stop_timeout must be positive")
	}

	return nil
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeProxyOnly, "proxy-only":
		return ModeProxyOnly
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

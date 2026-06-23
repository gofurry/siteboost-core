package systemdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
)

const (
	StateKind = "system_dns"

	stateVersion = 1
)

var (
	ErrNoState     = errors.New("no rollback state")
	ErrUnsupported = errors.New("system DNS is unsupported on this platform")
)

type Config struct {
	RollbackPath string   `json:"rollback_path"`
	Interfaces   []string `json:"interfaces"`
	ServerIPs    []string `json:"server_ips"`
}

type Result struct {
	Interfaces int      `json:"interfaces"`
	ServerIPs  []string `json:"server_ips,omitempty"`
	ViaHelper  bool     `json:"via_helper,omitempty"`
}

type State struct {
	Kind       string           `json:"kind"`
	Version    int              `json:"version"`
	OS         string           `json:"os"`
	Mode       string           `json:"mode"`
	Interfaces []InterfaceState `json:"interfaces"`
	ServerIPs  []string         `json:"server_ips"`
	CreatedAt  time.Time        `json:"created_at"`
}

type InterfaceState struct {
	InterfaceAlias   string   `json:"interface_alias"`
	InterfaceIndex   int      `json:"interface_index"`
	InterfaceGUID    string   `json:"interface_guid,omitempty"`
	AddressFamily    string   `json:"address_family"`
	Source           string   `json:"source"`
	Servers          []string `json:"servers,omitempty"`
	StaticServers    []string `json:"static_servers,omitempty"`
	StaticNameServer string   `json:"static_name_server,omitempty"`
}

type Platform interface {
	Name() string
	Snapshot(ctx context.Context, cfg Config) (State, error)
	Apply(ctx context.Context, cfg Config, state State) error
	Restore(ctx context.Context, state State) error
}

func ConfigFromApp(cfg config.Config) (Config, error) {
	host, _, err := net.SplitHostPort(cfg.DNS.ListenAddr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid dns_intercept listen_addr: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || ip.To4() == nil {
		return Config{}, fmt.Errorf("system DNS requires an IPv4 loopback DNS server")
	}
	out := Config{
		RollbackPath: cfg.Runtime.RollbackPath,
		Interfaces:   cleanStrings(cfg.DNS.Interfaces),
		ServerIPs:    []string{ip.String()},
	}
	return out, validateConfig(out)
}

func Preflight(ctx context.Context, cfg Config) (Result, error) {
	return PreflightWithPlatform(ctx, cfg, newOSPlatform())
}

func PreflightWithPlatform(ctx context.Context, cfg Config, platform Platform) (Result, error) {
	if platform == nil {
		return Result{}, fmt.Errorf("system DNS platform is required")
	}
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return Result{}, err
	}
	state, err := platform.Snapshot(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	state = completeState(state, platform.Name(), cfg)
	return resultFromState(state), nil
}

func Apply(ctx context.Context, cfg Config) (Result, error) {
	return ApplyWithPlatform(ctx, cfg, newOSPlatform())
}

func ApplyWithPlatform(ctx context.Context, cfg Config, platform Platform) (Result, error) {
	if platform == nil {
		return Result{}, fmt.Errorf("system DNS platform is required")
	}
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return Result{}, err
	}
	state, err := platform.Snapshot(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	state = completeState(state, platform.Name(), cfg)
	if err := WriteState(cfg.RollbackPath, state); err != nil {
		return Result{}, err
	}
	result := resultFromState(state)
	if err := platform.Apply(ctx, cfg, state); err != nil {
		return result, fmt.Errorf("apply system DNS: %w; rollback retained at %s", err, cfg.RollbackPath)
	}
	return result, nil
}

func Restore(ctx context.Context, rollbackPath string) (Result, error) {
	return RestoreWithPlatform(ctx, rollbackPath, newOSPlatform())
}

func RestoreWithPlatform(ctx context.Context, rollbackPath string, platform Platform) (Result, error) {
	if platform == nil {
		return Result{}, fmt.Errorf("system DNS platform is required")
	}
	state, err := ReadState(rollbackPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrNoState
		}
		return Result{}, err
	}
	if state.Kind != "" && state.Kind != StateKind {
		return Result{}, fmt.Errorf("rollback state kind %q is not %q", state.Kind, StateKind)
	}
	if state.Mode != "" && state.Mode != config.ModeDNS {
		return Result{}, fmt.Errorf("rollback state mode %q is not %q", state.Mode, config.ModeDNS)
	}
	if state.OS != "" && state.OS != platform.Name() {
		return Result{}, fmt.Errorf("rollback state was created for %s, current platform is %s", state.OS, platform.Name())
	}
	if err := platform.Restore(ctx, state); err != nil {
		return Result{}, err
	}
	if err := RemoveState(rollbackPath); err != nil {
		return Result{}, err
	}
	return resultFromState(state), nil
}

func HasState(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	state, err := ReadState(path)
	if err != nil {
		return false
	}
	return state.Kind == StateKind || state.Mode == config.ModeDNS
}

func WriteState(path string, state State) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("rollback path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create rollback directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rollback state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write rollback state: %w", err)
	}
	return nil
}

func ReadState(path string) (State, error) {
	var state State
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parse rollback state: %w", err)
	}
	return state, nil
}

func RemoveState(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func normalizeConfig(cfg Config) Config {
	cfg.RollbackPath = strings.TrimSpace(cfg.RollbackPath)
	cfg.Interfaces = cleanStrings(cfg.Interfaces)
	cfg.ServerIPs = cleanStrings(cfg.ServerIPs)
	return cfg
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.RollbackPath) == "" {
		return fmt.Errorf("rollback path is required")
	}
	if len(cfg.Interfaces) == 0 {
		return fmt.Errorf("system DNS interfaces are required")
	}
	for _, iface := range cfg.Interfaces {
		if strings.TrimSpace(iface) == "" {
			return fmt.Errorf("system DNS interfaces cannot contain empty values")
		}
	}
	if len(cfg.ServerIPs) == 0 {
		return fmt.Errorf("system DNS server IPs are required")
	}
	for _, server := range cfg.ServerIPs {
		ip := net.ParseIP(server)
		if ip == nil || !ip.IsLoopback() || ip.To4() == nil {
			return fmt.Errorf("system DNS server IP %q must be an IPv4 loopback address", server)
		}
	}
	return nil
}

func completeState(state State, platformName string, cfg Config) State {
	state.Kind = StateKind
	state.Version = stateVersion
	state.OS = platformName
	state.Mode = config.ModeDNS
	state.ServerIPs = append([]string(nil), cfg.ServerIPs...)
	state.CreatedAt = time.Now()
	return state
}

func resultFromState(state State) Result {
	return Result{
		Interfaces: len(state.Interfaces),
		ServerIPs:  append([]string(nil), state.ServerIPs...),
	}
}

func cleanStrings(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

type unsupportedPlatform struct {
	name string
}

func (p unsupportedPlatform) Name() string {
	if strings.TrimSpace(p.name) == "" {
		return runtime.GOOS
	}
	return p.name
}

func (p unsupportedPlatform) Snapshot(context.Context, Config) (State, error) {
	return State{}, fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}

func (p unsupportedPlatform) Apply(context.Context, Config, State) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}

func (p unsupportedPlatform) Restore(context.Context, State) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}

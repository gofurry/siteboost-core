package systemproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
)

const stateVersion = 1

var (
	ErrNoState     = errors.New("no rollback state")
	ErrUnsupported = errors.New("system proxy is unsupported on this platform")
)

type Config struct {
	Mode         string
	ProxyAddr    string
	PACURL       string
	RollbackPath string
	Services     []string
}

type State struct {
	Version   int           `json:"version"`
	OS        string        `json:"os"`
	Mode      string        `json:"mode"`
	ProxyAddr string        `json:"proxy_addr,omitempty"`
	PACURL    string        `json:"pac_url,omitempty"`
	Services  []string      `json:"services,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	Windows   *WindowsState `json:"windows,omitempty"`
	MacOS     *MacOSState   `json:"macos,omitempty"`
}

type Platform interface {
	Name() string
	Snapshot(ctx context.Context, cfg Config) (State, error)
	ApplyPAC(ctx context.Context, cfg Config) error
	ApplySystem(ctx context.Context, cfg Config) error
	Restore(ctx context.Context, state State) error
}

func ConfigFromApp(cfg config.Config, proxyAddr, pacURL string) Config {
	return Config{
		Mode:         cfg.Mode,
		ProxyAddr:    proxyAddr,
		PACURL:       pacURL,
		RollbackPath: cfg.Runtime.RollbackPath,
		Services:     append([]string(nil), cfg.System.Services...),
	}
}

func Apply(ctx context.Context, cfg Config) error {
	return ApplyWithPlatform(ctx, cfg, newOSPlatform())
}

func ApplyWithPlatform(ctx context.Context, cfg Config, platform Platform) error {
	if platform == nil {
		return fmt.Errorf("system proxy platform is required")
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	state, err := platform.Snapshot(ctx, cfg)
	if err != nil {
		return err
	}
	state.Version = stateVersion
	state.OS = platform.Name()
	state.Mode = cfg.Mode
	state.ProxyAddr = cfg.ProxyAddr
	state.PACURL = cfg.PACURL
	state.Services = append([]string(nil), cfg.Services...)
	state.CreatedAt = time.Now()

	if err := WriteState(cfg.RollbackPath, state); err != nil {
		return err
	}
	switch cfg.Mode {
	case config.ModePAC:
		return platform.ApplyPAC(ctx, cfg)
	case config.ModeSystem:
		return platform.ApplySystem(ctx, cfg)
	default:
		return fmt.Errorf("unsupported system proxy mode %q", cfg.Mode)
	}
}

func Restore(ctx context.Context, rollbackPath string) error {
	return RestoreWithPlatform(ctx, rollbackPath, newOSPlatform())
}

func RestoreWithPlatform(ctx context.Context, rollbackPath string, platform Platform) error {
	if platform == nil {
		return fmt.Errorf("system proxy platform is required")
	}
	state, err := ReadState(rollbackPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoState
		}
		return err
	}
	if state.OS != "" && state.OS != platform.Name() {
		return fmt.Errorf("rollback state was created for %s, current platform is %s", state.OS, platform.Name())
	}
	if err := platform.Restore(ctx, state); err != nil {
		return err
	}
	return RemoveState(rollbackPath)
}

func HasState(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
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

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.RollbackPath) == "" {
		return fmt.Errorf("rollback path is required")
	}
	switch cfg.Mode {
	case config.ModePAC:
		if strings.TrimSpace(cfg.PACURL) == "" {
			return fmt.Errorf("pac URL is required")
		}
		parsed, err := url.Parse(cfg.PACURL)
		if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
			return fmt.Errorf("invalid pac URL %q", cfg.PACURL)
		}
	case config.ModeSystem:
		if _, _, err := net.SplitHostPort(cfg.ProxyAddr); err != nil {
			return fmt.Errorf("invalid proxy address: %w", err)
		}
	default:
		return fmt.Errorf("unsupported system proxy mode %q", cfg.Mode)
	}
	return nil
}

type unsupportedPlatform struct {
	name string
}

func (p unsupportedPlatform) Name() string {
	return p.name
}

func (p unsupportedPlatform) Snapshot(context.Context, Config) (State, error) {
	return State{}, fmt.Errorf("%w: %s", ErrUnsupported, p.name)
}

func (p unsupportedPlatform) ApplyPAC(context.Context, Config) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.name)
}

func (p unsupportedPlatform) ApplySystem(context.Context, Config) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.name)
}

func (p unsupportedPlatform) Restore(context.Context, State) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.name)
}

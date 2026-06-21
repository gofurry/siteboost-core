package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
)

const (
	stateVersion = 1
	stateKind    = "hosts"

	markerStart = "# BEGIN steam-accelerator-core"
	markerEnd   = "# END steam-accelerator-core"
)

var (
	ErrNoState     = errors.New("no hosts rollback state")
	ErrUnsupported = errors.New("hosts mode is unsupported on this platform")
)

type Config struct {
	Path         string
	RollbackPath string
	MapIP        string
	Entries      []Entry
}

type Entry struct {
	IP   string `json:"ip"`
	Host string `json:"host"`
}

type State struct {
	Kind      string    `json:"kind"`
	Version   int       `json:"version"`
	OS        string    `json:"os"`
	Mode      string    `json:"mode"`
	Path      string    `json:"path"`
	MapIP     string    `json:"map_ip"`
	Entries   []Entry   `json:"entries"`
	CreatedAt time.Time `json:"created_at"`
}

type Platform interface {
	Name() string
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, mode os.FileMode) error
	FileMode(path string) (os.FileMode, error)
	CheckWritable(path string) error
}

func ConfigFromApp(cfg config.Config, entries []Entry) Config {
	return Config{
		Path:         cfg.Hosts.Path,
		RollbackPath: cfg.Runtime.RollbackPath,
		MapIP:        cfg.Hosts.MapIP,
		Entries:      append([]Entry(nil), entries...),
	}
}

func EntriesFromRules(compiled rules.CompiledRules, mapIP string) ([]Entry, []string, error) {
	domains := make([]string, 0, len(compiled.Exact))
	for _, entry := range compiled.Exact {
		domains = append(domains, entry.Host)
	}
	skipped := make([]string, 0, len(compiled.Wildcard))
	for _, entry := range compiled.Wildcard {
		skipped = append(skipped, entry.Rule)
	}
	entries, err := EntriesFromDomains(domains, mapIP)
	return entries, skipped, err
}

func EntriesFromDomains(domains []string, mapIP string) ([]Entry, error) {
	if strings.TrimSpace(mapIP) == "" {
		return nil, fmt.Errorf("hosts map IP is required")
	}
	seen := make(map[string]struct{}, len(domains))
	entries := make([]Entry, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		if strings.Contains(domain, "*") {
			return nil, fmt.Errorf("hosts cannot map wildcard domain %q", domain)
		}
		host, err := rules.NormalizeHost(domain)
		if err != nil {
			return nil, fmt.Errorf("normalize hosts domain %q: %w", domain, err)
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		entries = append(entries, Entry{IP: mapIP, Host: host})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Host != entries[j].Host {
			return entries[i].Host < entries[j].Host
		}
		return entries[i].IP < entries[j].IP
	})
	return entries, nil
}

func Apply(ctx context.Context, cfg Config) error {
	return ApplyWithPlatform(ctx, cfg, newOSPlatform())
}

func Preflight(ctx context.Context, cfg Config) error {
	return PreflightWithPlatform(ctx, cfg, newOSPlatform())
}

func PreflightWithPlatform(ctx context.Context, cfg Config, platform Platform) error {
	if platform == nil {
		return fmt.Errorf("hosts platform is required")
	}
	if platform.Name() != "windows" {
		return fmt.Errorf("%w: %s", ErrUnsupported, platform.Name())
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := platform.ReadFile(cfg.Path); err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}
	if err := platform.CheckWritable(cfg.Path); err != nil {
		return formatWriteError(err)
	}
	if err := checkRollbackWritable(cfg.RollbackPath); err != nil {
		return err
	}
	return nil
}

func ApplyWithPlatform(ctx context.Context, cfg Config, platform Platform) error {
	if platform == nil {
		return fmt.Errorf("hosts platform is required")
	}
	if platform.Name() != "windows" {
		return fmt.Errorf("%w: %s", ErrUnsupported, platform.Name())
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	original, err := platform.ReadFile(cfg.Path)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}
	mode, err := platform.FileMode(cfg.Path)
	if err != nil {
		mode = 0o644
	}
	next := appendManagedBlock(original, cfg.Entries)

	state := State{
		Kind:      stateKind,
		Version:   stateVersion,
		OS:        platform.Name(),
		Mode:      config.ModeHosts,
		Path:      cfg.Path,
		MapIP:     cfg.MapIP,
		Entries:   append([]Entry(nil), cfg.Entries...),
		CreatedAt: time.Now(),
	}
	if err := WriteState(cfg.RollbackPath, state); err != nil {
		return err
	}
	if err := platform.WriteFile(cfg.Path, next, mode); err != nil {
		_ = RemoveState(cfg.RollbackPath)
		return formatWriteError(err)
	}
	return nil
}

func Restore(ctx context.Context, rollbackPath string) error {
	return RestoreWithPlatform(ctx, rollbackPath, newOSPlatform())
}

func RestoreWithPlatform(ctx context.Context, rollbackPath string, platform Platform) error {
	if platform == nil {
		return fmt.Errorf("hosts platform is required")
	}
	state, err := ReadState(rollbackPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoState
		}
		return err
	}
	if state.Kind != stateKind && state.Mode != config.ModeHosts {
		return ErrNoState
	}
	if platform.Name() != "windows" {
		return fmt.Errorf("%w: %s", ErrUnsupported, platform.Name())
	}
	if state.OS != "" && state.OS != platform.Name() {
		return fmt.Errorf("hosts rollback state was created for %s, current platform is %s", state.OS, platform.Name())
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	body, err := platform.ReadFile(state.Path)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}
	mode, err := platform.FileMode(state.Path)
	if err != nil {
		mode = 0o644
	}
	next := removeManagedBlock(body)
	if err := platform.WriteFile(state.Path, next, mode); err != nil {
		return formatWriteError(err)
	}
	return RemoveState(rollbackPath)
}

func HasState(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	state, err := ReadState(path)
	if err != nil {
		return false
	}
	return state.Kind == stateKind || state.Mode == config.ModeHosts
}

func WriteState(path string, state State) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("hosts rollback path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create hosts rollback directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode hosts rollback state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write hosts rollback state: %w", err)
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
		return state, fmt.Errorf("parse hosts rollback state: %w", err)
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

func checkRollbackWritable(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create hosts rollback directory: %w", err)
	}
	probe, err := os.CreateTemp(dir, ".steam-accelerator-preflight-*")
	if err != nil {
		return fmt.Errorf("check hosts rollback directory writable: %w", err)
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return fmt.Errorf("check hosts rollback directory writable: %w", closeErr)
	}
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("check hosts rollback directory cleanup: %w", removeErr)
	}
	return nil
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("hosts path is required")
	}
	if strings.TrimSpace(cfg.RollbackPath) == "" {
		return fmt.Errorf("hosts rollback path is required")
	}
	if strings.TrimSpace(cfg.MapIP) == "" {
		return fmt.Errorf("hosts map IP is required")
	}
	if len(cfg.Entries) == 0 {
		return fmt.Errorf("hosts entries are required")
	}
	return nil
}

func appendManagedBlock(original []byte, entries []Entry) []byte {
	base := strings.TrimRight(string(removeManagedBlock(original)), "\r\n")
	block := renderBlock(entries)
	if base == "" {
		return []byte(block)
	}
	return []byte(base + "\n\n" + block)
}

func renderBlock(entries []Entry) string {
	var b strings.Builder
	b.WriteString(markerStart)
	b.WriteByte('\n')
	b.WriteString("# Managed by steam-accelerator-core. Do not edit this block manually.\n")
	for _, entry := range entries {
		b.WriteString(entry.IP)
		b.WriteByte(' ')
		b.WriteString(entry.Host)
		b.WriteByte('\n')
	}
	b.WriteString(markerEnd)
	b.WriteByte('\n')
	return b.String()
}

func removeManagedBlock(body []byte) []byte {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == markerStart {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == markerEnd {
				inBlock = false
			}
			continue
		}
		out = append(out, line)
	}
	joined := strings.Join(out, "\n")
	joined = strings.TrimRight(joined, "\n")
	if joined == "" {
		return nil
	}
	return []byte(joined + "\n")
}

func formatWriteError(err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("write hosts file: permission denied; run from an Administrator terminal")
	}
	return fmt.Errorf("write hosts file: %w", err)
}

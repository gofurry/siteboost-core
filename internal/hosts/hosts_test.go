package hosts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/rules"
)

type fakePlatform struct {
	name        string
	files       map[string][]byte
	mode        os.FileMode
	err         error
	writableErr error
}

func (p *fakePlatform) Name() string {
	if p.name == "" {
		return "windows"
	}
	return p.name
}

func (p *fakePlatform) ReadFile(path string) ([]byte, error) {
	if p.files == nil {
		p.files = make(map[string][]byte)
	}
	data, ok := p.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (p *fakePlatform) WriteFile(path string, data []byte, mode os.FileMode) error {
	if p.err != nil {
		return p.err
	}
	if p.files == nil {
		p.files = make(map[string][]byte)
	}
	p.files[path] = append([]byte(nil), data...)
	p.mode = mode
	return nil
}

func (p *fakePlatform) FileMode(string) (os.FileMode, error) {
	if p.mode == 0 {
		return 0o644, nil
	}
	return p.mode, nil
}

func (p *fakePlatform) CheckWritable(path string) error {
	if p.writableErr != nil {
		return p.writableErr
	}
	if p.files == nil {
		p.files = make(map[string][]byte)
	}
	if _, ok := p.files[path]; !ok {
		return os.ErrNotExist
	}
	return nil
}

func TestEntriesFromRulesSkipsWildcards(t *testing.T) {
	matcher, err := rules.NewMatcher(rules.DefaultSteamRules, []string{"login.steampowered.com"})
	if err != nil {
		t.Fatal(err)
	}
	entries, skipped, err := EntriesFromRules(matcher.Rules(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) == 0 {
		t.Fatalf("wildcard rules were not reported as skipped")
	}
	hosts := make(map[string]bool)
	for _, entry := range entries {
		hosts[entry.Host] = true
	}
	if !hosts["store.steampowered.com"] || !hosts["login.steampowered.com"] {
		t.Fatalf("entries missing expected hosts: %#v", entries)
	}
	if hosts["*.steamcommunity.com"] {
		t.Fatalf("wildcard host should not be mapped")
	}
}

func TestApplyAndRestoreMarkerBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	rollback := filepath.Join(t.TempDir(), "rollback.json")
	platform := &fakePlatform{files: map[string][]byte{
		path: []byte("127.0.0.1 localhost\n\n# user content\n"),
	}}
	cfg := Config{
		Path:         path,
		RollbackPath: rollback,
		MapIP:        "127.0.0.1",
		Entries: []Entry{
			{IP: "127.0.0.1", Host: "store.steampowered.com"},
		},
	}
	if err := ApplyWithPlatform(context.Background(), cfg, platform); err != nil {
		t.Fatal(err)
	}
	body := string(platform.files[path])
	if !strings.Contains(body, markerStart) || !strings.Contains(body, "store.steampowered.com") {
		t.Fatalf("hosts block not written:\n%s", body)
	}
	if !strings.Contains(body, "# user content") {
		t.Fatalf("user content was lost:\n%s", body)
	}
	if !HasState(rollback) {
		t.Fatalf("rollback state was not written")
	}

	if err := RestoreWithPlatform(context.Background(), rollback, platform); err != nil {
		t.Fatal(err)
	}
	body = string(platform.files[path])
	if strings.Contains(body, markerStart) || strings.Contains(body, "store.steampowered.com") {
		t.Fatalf("hosts block was not removed:\n%s", body)
	}
	if !strings.Contains(body, "# user content") {
		t.Fatalf("user content was not preserved:\n%s", body)
	}
	if HasState(rollback) {
		t.Fatalf("rollback state still exists")
	}
}

func TestApplyUnsupportedPlatform(t *testing.T) {
	err := ApplyWithPlatform(context.Background(), Config{
		Path:         "hosts",
		RollbackPath: filepath.Join(t.TempDir(), "rollback.json"),
		MapIP:        "127.0.0.1",
		Entries:      []Entry{{IP: "127.0.0.1", Host: "store.steampowered.com"}},
	}, &fakePlatform{name: "linux"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestPreflightChecksHostsWritable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	rollback := filepath.Join(t.TempDir(), "rollback.json")
	platform := &fakePlatform{
		files:       map[string][]byte{path: []byte("127.0.0.1 localhost\n")},
		writableErr: os.ErrPermission,
	}
	err := PreflightWithPlatform(context.Background(), Config{
		Path:         path,
		RollbackPath: rollback,
		MapIP:        "127.0.0.1",
		Entries:      []Entry{{IP: "127.0.0.1", Host: "store.steampowered.com"}},
	}, platform)
	if err == nil {
		t.Fatalf("PreflightWithPlatform returned nil")
	}
	if !strings.Contains(err.Error(), "Administrator terminal") {
		t.Fatalf("err = %v, want Administrator hint", err)
	}
	if HasState(rollback) {
		t.Fatalf("preflight should not create rollback state")
	}
}

package systemdns

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAndRestoreWithFakePlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	platform := &fakePlatform{name: "fake", snapshot: State{Interfaces: []InterfaceState{{
		InterfaceAlias: "Wi-Fi",
		InterfaceIndex: 12,
		AddressFamily:  "IPv4",
		Source:         "dhcp",
	}}}}
	cfg := Config{
		RollbackPath: path,
		Interfaces:   []string{"Wi-Fi"},
		ServerIPs:    []string{"127.0.0.1"},
	}
	result, err := ApplyWithPlatform(context.Background(), cfg, platform)
	if err != nil {
		t.Fatal(err)
	}
	if result.Interfaces != 1 {
		t.Fatalf("interfaces = %d, want 1", result.Interfaces)
	}
	if !HasState(path) {
		t.Fatalf("rollback state was not written")
	}
	if len(platform.applied.ServerIPs) != 1 || platform.applied.ServerIPs[0] != "127.0.0.1" {
		t.Fatalf("applied config = %#v", platform.applied)
	}
	if _, err := RestoreWithPlatform(context.Background(), path, platform); err != nil {
		t.Fatal(err)
	}
	if !platform.restored {
		t.Fatalf("platform restore was not called")
	}
	if HasState(path) {
		t.Fatalf("rollback state was not removed")
	}
}

func TestApplyFailureRetainsRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	platform := &fakePlatform{
		name: "fake",
		snapshot: State{Interfaces: []InterfaceState{{
			InterfaceAlias: "Ethernet",
			InterfaceIndex: 8,
			AddressFamily:  "IPv4",
			Source:         "static",
			StaticServers:  []string{"1.1.1.1"},
		}}},
		applyErr: errors.New("Access is denied."),
	}
	_, err := ApplyWithPlatform(context.Background(), Config{
		RollbackPath: path,
		Interfaces:   []string{"Ethernet"},
		ServerIPs:    []string{"127.0.0.1"},
	}, platform)
	if err == nil || !strings.Contains(err.Error(), "rollback retained") {
		t.Fatalf("ApplyWithPlatform() error = %v, want retained rollback guidance", err)
	}
	if !HasState(path) {
		t.Fatalf("rollback should remain after apply failure")
	}
}

func TestRestoreNoState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	_, err := RestoreWithPlatform(context.Background(), path, &fakePlatform{name: "fake"})
	if !errors.Is(err, ErrNoState) {
		t.Fatalf("err = %v, want ErrNoState", err)
	}
}

func TestUnsupportedPlatformDoesNotWriteRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	_, err := ApplyWithPlatform(context.Background(), Config{
		RollbackPath: path,
		Interfaces:   []string{"Wi-Fi"},
		ServerIPs:    []string{"127.0.0.1"},
	}, unsupportedPlatform{name: "linux"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("rollback state should not exist, stat err = %v", statErr)
	}
}

func TestWindowsPlatformSnapshotApplyAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	runner := &fakePowerShellRunner{output: `[
  {"InterfaceAlias":"Wi-Fi","InterfaceIndex":12,"InterfaceGuid":"{AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE}","AddressFamily":"IPv4","StaticNameServer":"","Servers":["192.0.2.53"]},
  {"InterfaceAlias":"Ethernet","InterfaceIndex":13,"InterfaceGuid":"FFFFFFFF-BBBB-CCCC-DDDD-EEEEEEEEEEEE","AddressFamily":"IPv4","StaticNameServer":"1.1.1.1,8.8.8.8","Servers":["1.1.1.1","8.8.8.8"]}
]`}
	platform := windowsPlatform{runner: runner}
	cfg := Config{
		RollbackPath: path,
		Interfaces:   []string{"Wi-Fi", "13"},
		ServerIPs:    []string{"127.0.0.1"},
	}
	result, err := ApplyWithPlatform(context.Background(), cfg, platform)
	if err != nil {
		t.Fatal(err)
	}
	if result.Interfaces != 2 {
		t.Fatalf("interfaces = %d, want 2", result.Interfaces)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want snapshot+apply", len(runner.calls))
	}
	if !strings.Contains(runner.calls[1], "Set-DnsClientServerAddress") || !strings.Contains(runner.calls[1], "127.0.0.1") {
		t.Fatalf("apply script = %s", runner.calls[1])
	}
	if _, err := RestoreWithPlatform(context.Background(), path, platform); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("calls = %d, want restore call", len(runner.calls))
	}
	restoreScript := runner.calls[2]
	for _, want := range []string{"-ResetServerAddresses", "1.1.1.1", "8.8.8.8"} {
		if !strings.Contains(restoreScript, want) {
			t.Fatalf("restore script = %s, want %q", restoreScript, want)
		}
	}
}

func TestWindowsPlatformRejectsUnknownInterface(t *testing.T) {
	runner := &fakePowerShellRunner{output: `[{"InterfaceAlias":"Wi-Fi","InterfaceIndex":12,"AddressFamily":"IPv4","StaticNameServer":"","Servers":["192.0.2.53"]}]`}
	_, err := PreflightWithPlatform(context.Background(), Config{
		RollbackPath: filepath.Join(t.TempDir(), "rollback.json"),
		Interfaces:   []string{"Ethernet"},
		ServerIPs:    []string{"127.0.0.1"},
	}, windowsPlatform{runner: runner})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("PreflightWithPlatform() error = %v, want unknown interface", err)
	}
}

type fakePlatform struct {
	name     string
	snapshot State
	applied  Config
	restored bool
	applyErr error
}

func (p *fakePlatform) Name() string { return p.name }

func (p *fakePlatform) Snapshot(context.Context, Config) (State, error) {
	return p.snapshot, nil
}

func (p *fakePlatform) Apply(_ context.Context, cfg Config, _ State) error {
	p.applied = cfg
	return p.applyErr
}

func (p *fakePlatform) Restore(context.Context, State) error {
	p.restored = true
	return nil
}

type fakePowerShellRunner struct {
	output string
	calls  []string
}

func (r *fakePowerShellRunner) RunPowerShell(_ context.Context, script string) (string, error) {
	r.calls = append(r.calls, script)
	return r.output, nil
}

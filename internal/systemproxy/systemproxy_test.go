package systemproxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/config"
)

func TestApplyAndRestoreWithFakePlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	platform := &fakePlatform{name: "fake"}
	cfg := Config{
		Mode:         config.ModePAC,
		PACURL:       "http://127.0.0.1:26502/proxy.pac",
		ProxyAddr:    "127.0.0.1:26501",
		RollbackPath: path,
	}
	if err := ApplyWithPlatform(context.Background(), cfg, platform); err != nil {
		t.Fatal(err)
	}
	if platform.applied != config.ModePAC {
		t.Fatalf("applied = %q", platform.applied)
	}
	if !HasState(path) {
		t.Fatalf("rollback state was not written")
	}
	if err := RestoreWithPlatform(context.Background(), path, platform); err != nil {
		t.Fatal(err)
	}
	if !platform.restored {
		t.Fatalf("platform restore was not called")
	}
	if HasState(path) {
		t.Fatalf("rollback state was not removed")
	}
}

func TestRestoreNoState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	err := RestoreWithPlatform(context.Background(), path, &fakePlatform{name: "fake"})
	if !errors.Is(err, ErrNoState) {
		t.Fatalf("err = %v, want ErrNoState", err)
	}
}

func TestUnsupportedPlatformDoesNotWriteRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	err := ApplyWithPlatform(context.Background(), Config{
		Mode:         config.ModeSystem,
		ProxyAddr:    "127.0.0.1:26501",
		RollbackPath: path,
	}, unsupportedPlatform{name: "linux"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("rollback state should not exist, stat err = %v", statErr)
	}
}

func TestWindowsPlatformUsesFakeRegistryBackend(t *testing.T) {
	backend := &fakeWindowsBackend{state: WindowsState{
		ProxyEnable:   WindowsDWORD{Exists: true, Value: 0},
		ProxyServer:   WindowsString{Exists: true, Value: "old:8080"},
		AutoConfigURL: WindowsString{Exists: false},
	}}
	platform := windowsPlatform{backend: backend}
	state, err := platform.Snapshot(context.Background(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Windows == nil || !state.Windows.ProxyServer.Exists {
		t.Fatalf("snapshot = %#v", state)
	}
	if err := platform.ApplySystem(context.Background(), Config{ProxyAddr: "127.0.0.1:26501"}); err != nil {
		t.Fatal(err)
	}
	if backend.proxyAddr != "127.0.0.1:26501" {
		t.Fatalf("proxy addr = %q", backend.proxyAddr)
	}
	if err := platform.Restore(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(backend.restored, backend.state) {
		t.Fatalf("restored = %#v, want %#v", backend.restored, backend.state)
	}
}

func TestMacOSPlatformNetworkSetupCommands(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"-listallnetworkservices":              "An asterisk (*) denotes that a network service is disabled.\nWi-Fi\n*Bluetooth PAN\nUSB 10/100 LAN\n",
		"-getwebproxy\x00Wi-Fi":                "Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n",
		"-getsecurewebproxy\x00Wi-Fi":          "Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n",
		"-getautoproxyurl\x00Wi-Fi":            "URL:\nEnabled: No\n",
		"-getwebproxy\x00USB 10/100 LAN":       "Enabled: Yes\nServer: old.local\nPort: 8080\nAuthenticated Proxy Enabled: 0\n",
		"-getsecurewebproxy\x00USB 10/100 LAN": "Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n",
		"-getautoproxyurl\x00USB 10/100 LAN":   "URL: http://old/proxy.pac\nEnabled: Yes\n",
	}}
	platform := macOSPlatform{runner: runner}
	state, err := platform.Snapshot(context.Background(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if state.MacOS == nil || len(state.MacOS.Services) != 2 {
		t.Fatalf("snapshot = %#v", state)
	}
	if err := platform.ApplyPAC(context.Background(), Config{PACURL: "http://127.0.0.1:26502/proxy.pac"}); err != nil {
		t.Fatal(err)
	}
	if !runner.called("-setautoproxyurl", "Wi-Fi", "http://127.0.0.1:26502/proxy.pac") {
		t.Fatalf("PAC URL command not called: %#v", runner.calls)
	}
	if !runner.called("-setsecurewebproxystate", "USB 10/100 LAN", "off") {
		t.Fatalf("secure proxy off command not called: %#v", runner.calls)
	}
	if err := platform.ApplySystem(context.Background(), Config{ProxyAddr: "127.0.0.1:26501", Services: []string{"Wi-Fi"}}); err != nil {
		t.Fatal(err)
	}
	if !runner.called("-setwebproxy", "Wi-Fi", "127.0.0.1", "26501") {
		t.Fatalf("web proxy command not called: %#v", runner.calls)
	}
}

func TestMacOSPlatformRejectsAuthenticatedProxy(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"-getwebproxy\x00Wi-Fi":       "Enabled: Yes\nServer: old.local\nPort: 8080\nAuthenticated Proxy Enabled: 1\n",
		"-getsecurewebproxy\x00Wi-Fi": "Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n",
		"-getautoproxyurl\x00Wi-Fi":   "URL:\nEnabled: No\n",
	}}
	platform := macOSPlatform{runner: runner}
	_, err := platform.Snapshot(context.Background(), Config{Services: []string{"Wi-Fi"}})
	if err == nil || !strings.Contains(err.Error(), "authenticated proxy") {
		t.Fatalf("err = %v", err)
	}
}

type fakePlatform struct {
	name     string
	applied  string
	restored bool
}

func (p *fakePlatform) Name() string { return p.name }

func (p *fakePlatform) Snapshot(context.Context, Config) (State, error) {
	return State{}, nil
}

func (p *fakePlatform) ApplyPAC(context.Context, Config) error {
	p.applied = config.ModePAC
	return nil
}

func (p *fakePlatform) ApplySystem(context.Context, Config) error {
	p.applied = config.ModeSystem
	return nil
}

func (p *fakePlatform) Restore(context.Context, State) error {
	p.restored = true
	return nil
}

type fakeWindowsBackend struct {
	state     WindowsState
	proxyAddr string
	pacURL    string
	restored  WindowsState
}

func (b *fakeWindowsBackend) Snapshot() (WindowsState, error) { return b.state, nil }

func (b *fakeWindowsBackend) ApplyPAC(pacURL string) error {
	b.pacURL = pacURL
	return nil
}

func (b *fakeWindowsBackend) ApplySystem(proxyAddr string) error {
	b.proxyAddr = proxyAddr
	return nil
}

func (b *fakeWindowsBackend) Restore(state WindowsState) error {
	b.restored = state
	return nil
}

type fakeRunner struct {
	outputs map[string]string
	calls   [][]string
}

func (r *fakeRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	if out, ok := r.outputs[strings.Join(args, "\x00")]; ok {
		return out, nil
	}
	return "", nil
}

func (r *fakeRunner) called(args ...string) bool {
	for _, call := range r.calls {
		if reflect.DeepEqual(call, args) {
			return true
		}
	}
	return false
}

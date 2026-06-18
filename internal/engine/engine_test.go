package engine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
)

func TestEngineStartStopStatus(t *testing.T) {
	cfg := config.Default()
	cfg.Proxy.ListenAddr = "127.0.0.1:0"
	cfg.Runtime.StatePath = t.TempDir() + "/runtime.json"

	eng, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	status := eng.Status()
	if !status.Running {
		t.Fatalf("engine should be running")
	}
	if status.ListenAddr == "" {
		t.Fatalf("listen addr is empty")
	}
	if status.RuleCount == 0 {
		t.Fatalf("rule count is zero")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if eng.Status().Running {
		t.Fatalf("engine should be stopped")
	}
}

func TestEnginePACModeStartsPACAndRestores(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModePAC
	cfg.Proxy.ListenAddr = "127.0.0.1:0"
	cfg.PAC.ListenAddr = "127.0.0.1:0"
	cfg.Runtime.StatePath = t.TempDir() + "/runtime.json"
	cfg.Runtime.RollbackPath = t.TempDir() + "/rollback.json"

	var applied systemproxy.Config
	var restoredPath string
	restoreFns := replaceSystemProxyHooks(
		func(ctx context.Context, cfg systemproxy.Config) error {
			applied = cfg
			return nil
		},
		func(ctx context.Context, path string) error {
			restoredPath = path
			return nil
		},
		func(path string) bool { return true },
	)
	defer restoreFns()

	eng, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	status := eng.Status()
	if status.PACURL == "" {
		t.Fatalf("pac url is empty")
	}
	if !status.Rollback {
		t.Fatalf("rollback status should be true")
	}
	if applied.Mode != config.ModePAC || applied.PACURL != status.PACURL {
		t.Fatalf("applied system proxy = %#v, status = %#v", applied, status)
	}

	resp, err := http.Get(status.PACURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "PROXY "+status.ListenAddr) {
		t.Fatalf("PAC does not point at proxy %s:\n%s", status.ListenAddr, body)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if restoredPath != cfg.Runtime.RollbackPath {
		t.Fatalf("restored path = %q, want %q", restoredPath, cfg.Runtime.RollbackPath)
	}
}

func TestEngineSystemModeAppliesManualProxy(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeSystem
	cfg.Proxy.ListenAddr = "127.0.0.1:0"
	cfg.Runtime.RollbackPath = t.TempDir() + "/rollback.json"

	var applied systemproxy.Config
	restoreFns := replaceSystemProxyHooks(
		func(ctx context.Context, cfg systemproxy.Config) error {
			applied = cfg
			return nil
		},
		func(ctx context.Context, path string) error { return nil },
		func(path string) bool { return false },
	)
	defer restoreFns()

	eng, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	status := eng.Status()
	if status.PACURL != "" {
		t.Fatalf("system mode should not expose PAC URL: %q", status.PACURL)
	}
	if applied.Mode != config.ModeSystem || applied.ProxyAddr != status.ListenAddr {
		t.Fatalf("applied system proxy = %#v, status = %#v", applied, status)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func replaceSystemProxyHooks(apply func(context.Context, systemproxy.Config) error, restore func(context.Context, string) error, has func(string) bool) func() {
	oldApply := applySystemProxy
	oldRestore := restoreSystemProxy
	oldHas := hasRollbackState
	applySystemProxy = apply
	restoreSystemProxy = restore
	hasRollbackState = has
	return func() {
		applySystemProxy = oldApply
		restoreSystemProxy = oldRestore
		hasRollbackState = oldHas
	}
}

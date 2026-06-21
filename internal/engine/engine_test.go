package engine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/hosts"
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

func TestEffectiveResolverConfigHostsModeUsesDoHByDefault(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Resolver.Mode = config.ResolverSystem

	got := effectiveResolverConfig(cfg)
	if got.Mode != config.ResolverDoH {
		t.Fatalf("resolver mode = %q, want %q", got.Mode, config.ResolverDoH)
	}
	if len(got.Servers) == 0 {
		t.Fatalf("DoH servers were not filled")
	}
}

func TestEffectiveResolverConfigKeepsExplicitResolver(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Resolver.Mode = config.ResolverUDP
	cfg.Resolver.Servers = []string{"1.1.1.1:53"}

	got := effectiveResolverConfig(cfg)
	if got.Mode != config.ResolverUDP {
		t.Fatalf("resolver mode = %q, want %q", got.Mode, config.ResolverUDP)
	}
	if len(got.Servers) != 1 || got.Servers[0] != "1.1.1.1:53" {
		t.Fatalf("resolver servers = %#v", got.Servers)
	}
}

func TestEffectiveResolverConfigKeepsSystemResolverWithProxyUpstream(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Upstream.Type = config.UpstreamHTTP
	cfg.Upstream.Address = "127.0.0.1:18080"

	got := effectiveResolverConfig(cfg)
	if got.Mode != config.ResolverSystem {
		t.Fatalf("resolver mode = %q, want %q", got.Mode, config.ResolverSystem)
	}
}

func TestEngineHostsModeStartsReverseAndRestoresHosts(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.Runtime.RollbackPath = t.TempDir() + "/rollback.json"

	var applied hosts.Config
	var restoredPath string
	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error {
			return nil
		},
		func(ctx context.Context, cfg hosts.Config) error {
			applied = cfg
			return nil
		},
		func(ctx context.Context, path string) error {
			restoredPath = path
			return nil
		},
		func(ctx context.Context, cfg certstore.Config) (bool, error) {
			return true, nil
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
	if status.HostsHTTP == "" || status.HostsHTTPS == "" {
		t.Fatalf("hosts reverse addrs are empty: %#v", status)
	}
	if !status.CertInstalled {
		t.Fatalf("cert should be reported as installed")
	}
	if !status.Rollback {
		t.Fatalf("rollback should be reported")
	}
	if status.ResolverMode != config.ResolverDoH {
		t.Fatalf("resolver mode = %q, want %q", status.ResolverMode, config.ResolverDoH)
	}
	if len(status.ResolverServers) == 0 {
		t.Fatalf("resolver servers were not reported")
	}
	if len(applied.Entries) == 0 {
		t.Fatalf("hosts entries were not applied")
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

func replaceHostsHooks(preflight func(context.Context, hosts.Config) error, apply func(context.Context, hosts.Config) error, restore func(context.Context, string) error, certCheck func(context.Context, certstore.Config) (bool, error), has func(string) bool) func() {
	oldPreflight := preflightHosts
	oldApply := applyHosts
	oldRestore := restoreHosts
	oldCertCheck := isCertInstalled
	oldHas := hasRollbackState
	preflightHosts = preflight
	applyHosts = apply
	restoreHosts = restore
	isCertInstalled = certCheck
	hasRollbackState = has
	return func() {
		preflightHosts = oldPreflight
		applyHosts = oldApply
		restoreHosts = oldRestore
		isCertInstalled = oldCertCheck
		hasRollbackState = oldHas
	}
}

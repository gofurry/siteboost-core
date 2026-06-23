package engine

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/dnsintercept"
	"github.com/gofurry/go-steam-core/internal/hosts"
	"github.com/gofurry/go-steam-core/internal/privilege"
	"github.com/gofurry/go-steam-core/internal/provider"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/systemdns"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
	"github.com/gofurry/go-steam-core/internal/upstream"
	"github.com/miekg/dns"
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
	if status.RuleSetName != provider.SteamRuleSetName || status.RuleSetVersion == "" {
		t.Fatalf("rule set was not reported: %#v", status)
	}
	if len(status.Providers) != 1 || status.Providers[0].ID != provider.IDSteam {
		t.Fatalf("providers were not reported: %#v", status.Providers)
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

func TestEngineGitHubProviderOnlyReportsExperimental(t *testing.T) {
	cfg := config.Default()
	cfg.Providers.Enabled = []string{provider.IDGitHub}
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
	if status.RuleSetName != provider.GitHubRuleSetName || status.RuleSetVersion == "" {
		t.Fatalf("github rule set was not reported: %#v", status)
	}
	if status.UpstreamProfiles != 0 {
		t.Fatalf("github skeleton should not report default upstream profiles: %#v", status)
	}
	if len(status.Providers) != 1 || status.Providers[0].ID != provider.IDGitHub || status.Providers[0].Status != provider.StatusExperimental {
		t.Fatalf("providers were not reported: %#v", status.Providers)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
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

func TestEffectiveResolverConfigDNSModeUsesDoHByDefault(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeDNS
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

func TestEngineDNSModeStartsManualServerWithoutSystemWrites(t *testing.T) {
	upstreamAddr, stopUpstream := startEngineTestDNSServer(t)
	defer stopUpstream()

	cfg := config.Default()
	cfg.Mode = config.ModeDNS
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.DNS.ListenAddr = "127.0.0.1:0"
	cfg.DNS.MapIPv4 = "127.0.0.1"
	cfg.Resolver.Mode = config.ResolverUDP
	cfg.Resolver.Servers = []string{upstreamAddr}
	cfg.Cert.Dir = t.TempDir()

	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("preflightHosts should not be called in dns mode")
			return nil
		},
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("applyHosts should not be called in dns mode")
			return nil
		},
		func(ctx context.Context, path string) error {
			t.Fatalf("restoreHosts should not be called in dns mode")
			return nil
		},
		func(ctx context.Context, cfg certstore.Config) (bool, error) {
			t.Fatalf("isCertInstalled should not be called in dns mode")
			return false, nil
		},
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			t.Fatalf("ensureCertTrusted should not be called in dns mode")
			return certstore.TrustResult{}, nil
		},
		func(path string) bool { return false },
	)
	defer restoreFns()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if status.DNSIntercept == nil {
		t.Fatalf("dns intercept status missing: %#v", status)
	}
	if status.DNSIntercept.SystemDNS {
		t.Fatalf("system DNS should not be reported as modified")
	}
	if status.HostsHTTP == "" || status.HostsHTTPS == "" {
		t.Fatalf("reverse proxy addrs missing: %#v", status)
	}
	if status.CertInstalled {
		t.Fatalf("cert should not be reported as installed in dns mode")
	}
	if len(status.SystemChanges) != 0 {
		t.Fatalf("dns mode should not report system writes: %#v", status.SystemChanges)
	}
	if status.UpstreamProfiles == 0 {
		t.Fatalf("default provider profiles should be available in dns mode")
	}

	targetResp := exchangeEngineDNS(t, status.DNSIntercept.ListenAddr, "steamcommunity.com.", dns.TypeA)
	if len(targetResp.Answer) != 1 || targetResp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Fatalf("target response = %#v", targetResp.Answer)
	}
	forwardedResp := exchangeEngineDNS(t, status.DNSIntercept.ListenAddr, "example.test.", dns.TypeA)
	if len(forwardedResp.Answer) != 1 || forwardedResp.Answer[0].(*dns.A).A.String() != "192.0.2.88" {
		t.Fatalf("forwarded response = %#v", forwardedResp.Answer)
	}

	status = eng.Status()
	if status.DNSIntercept.TargetQueries != 1 || status.DNSIntercept.ForwardedQueries != 1 {
		t.Fatalf("dns status = %#v", status.DNSIntercept)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestEngineReportsPageEnhanceStatus(t *testing.T) {
	upstreamAddr, stopUpstream := startEngineTestDNSServer(t)
	defer stopUpstream()

	cfg := config.Default()
	cfg.Mode = config.ModeDNS
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.DNS.ListenAddr = "127.0.0.1:0"
	cfg.Resolver.Mode = config.ResolverUDP
	cfg.Resolver.Servers = []string{upstreamAddr}
	cfg.Cert.Dir = t.TempDir()
	cfg.PageEnhance.Enabled = true
	cfg.PageEnhance.Transforms = []config.PageEnhanceTransformConfig{{
		Name: "status",
		Headers: config.PageEnhanceHeadersConfig{
			Set: map[string]string{"X-Enhanced": "yes"},
		},
	}}

	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if status.PageEnhance == nil || !status.PageEnhance.Enabled || status.PageEnhance.Transforms != 1 {
		t.Fatalf("page enhance status = %#v", status.PageEnhance)
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestEngineDNSSystemStrategyAppliesAndRestoresSystemDNS(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeDNS
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.DNS.Strategy = config.DNSInterceptSystem
	cfg.DNS.ListenAddr = "127.0.0.1:53"
	cfg.DNS.Interfaces = []string{"Wi-Fi"}
	cfg.Cert.Dir = t.TempDir()
	cfg.Runtime.RollbackPath = t.TempDir() + "/rollback.json"

	var sequence []string
	restoreDNS := replaceSystemDNSHooks(
		func(ctx context.Context, cfg systemdns.Config) (systemdns.Result, error) {
			sequence = append(sequence, "preflight")
			if len(cfg.Interfaces) != 1 || cfg.Interfaces[0] != "Wi-Fi" {
				t.Fatalf("system DNS interfaces = %#v", cfg.Interfaces)
			}
			if len(cfg.ServerIPs) != 1 || cfg.ServerIPs[0] != "127.0.0.1" {
				t.Fatalf("system DNS server IPs = %#v", cfg.ServerIPs)
			}
			return systemdns.Result{Interfaces: 1, ServerIPs: cfg.ServerIPs}, nil
		},
		func(ctx context.Context, cfg systemdns.Config) (systemdns.Result, error) {
			sequence = append(sequence, "apply")
			return systemdns.Result{Interfaces: 1, ServerIPs: cfg.ServerIPs}, nil
		},
		func(ctx context.Context, path string) error {
			sequence = append(sequence, "restore")
			if path != cfg.Runtime.RollbackPath {
				t.Fatalf("restore path = %q, want %q", path, cfg.Runtime.RollbackPath)
			}
			return nil
		},
	)
	defer restoreDNS()
	restoreDNSIntercept := replaceDNSInterceptHook(func(dnsCfg dnsintercept.Config, matcher *rules.Matcher, forwarder dnsintercept.Forwarder) (dnsInterceptServer, error) {
		return &fakeDNSInterceptServer{
			status: dnsintercept.Status{Strategy: dnsCfg.Strategy, ListenAddr: dnsCfg.ListenAddr},
			onStart: func() {
				sequence = append(sequence, "dns_start")
			},
			onStop: func() {
				sequence = append(sequence, "dns_stop")
			},
		}, nil
	})
	defer restoreDNSIntercept()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if status.DNSIntercept == nil || !status.DNSIntercept.SystemDNS {
		t.Fatalf("system DNS status missing: %#v", status.DNSIntercept)
	}
	if !hasSystemChange(status.SystemChanges, "system_dns", "preflight", "ok") ||
		!hasSystemChange(status.SystemChanges, "system_dns", "apply", "ok") {
		t.Fatalf("system DNS changes missing: %#v", status.SystemChanges)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	want := []string{"preflight", "dns_start", "apply", "restore", "dns_stop"}
	if strings.Join(sequence, ",") != strings.Join(want, ",") {
		t.Fatalf("sequence = %#v, want %#v", sequence, want)
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
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			return certstore.TrustResult{AlreadyTrusted: true}, nil
		},
		func(path string) bool { return true },
	)
	defer restoreFns()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return []upstream.ProbeResult{{
			ProviderID: provider.IDSteam,
			Host:       "steamcommunity.com",
			Target:     "steamcommunity-a.akamaihd.net",
			OK:         true,
			Stage:      "http",
			HTTPStatus: "200 OK",
		}}
	})
	defer restoreProbe()

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
	if status.UpstreamProfiles == 0 {
		t.Fatalf("default upstream profiles were not reported")
	}
	if len(status.Providers) != 1 || status.Providers[0].ID != provider.IDSteam || status.Providers[0].Status != provider.StatusStable {
		t.Fatalf("providers were not reported: %#v", status.Providers)
	}
	if len(status.SystemChanges) == 0 {
		t.Fatalf("system changes were not reported")
	}
	if len(status.StartupProbes) != 1 || !status.StartupProbes[0].OK {
		t.Fatalf("startup probes were not reported: %#v", status.StartupProbes)
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

func TestEngineHostsModeAutoInstallsCert(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.Runtime.RollbackPath = t.TempDir() + "/rollback.json"

	installed := false
	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error { return nil },
		func(ctx context.Context, cfg hosts.Config) error { return nil },
		func(ctx context.Context, path string) error { return nil },
		func(ctx context.Context, cfg certstore.Config) (bool, error) {
			return installed, nil
		},
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			installed = true
			return certstore.TrustResult{StoreScope: cfg.StoreScope, Installed: true, Changed: true}, nil
		},
		func(path string) bool { return true },
	)
	defer restoreFns()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if !status.CertInstalled {
		t.Fatalf("cert should be installed after auto install")
	}
	if !hasSystemChange(status.SystemChanges, "root_ca", "install", "ok") {
		t.Fatalf("root CA install change missing: %#v", status.SystemChanges)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestEngineHostsModeAutoInstallCanBeDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Cert.AutoInstall = false

	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error { return nil },
		func(ctx context.Context, cfg hosts.Config) error { return nil },
		func(ctx context.Context, path string) error { return nil },
		func(ctx context.Context, cfg certstore.Config) (bool, error) { return false, nil },
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			t.Fatalf("ensureCertTrusted should not be called when auto install is disabled")
			return certstore.TrustResult{}, nil
		},
		func(path string) bool { return false },
	)
	defer restoreFns()

	eng, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "cert.auto_install") {
		t.Fatalf("Start error = %v, want cert.auto_install guidance", err)
	}
}

func TestEngineHostsModeUsesElevatedHelper(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.Cert.Dir = t.TempDir()
	cfg.Runtime.RollbackPath = config.DefaultRollbackPath()

	var helperCalled bool
	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("preflightHosts should be handled by elevated helper")
			return nil
		},
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("applyHosts should be handled by elevated helper")
			return nil
		},
		func(ctx context.Context, path string) error { return nil },
		func(ctx context.Context, cfg certstore.Config) (bool, error) {
			t.Fatalf("isCertInstalled should be handled by elevated helper")
			return false, nil
		},
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			t.Fatalf("ensureCertTrusted should be handled by elevated helper")
			return certstore.TrustResult{}, nil
		},
		func(path string) bool { return true },
	)
	defer restoreFns()
	oldPrepare := prepareHostsElevated
	oldShouldUseHelper := shouldUseHostHelper
	prepareHostsElevated = func(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (privilege.PrepareHostsResult, error) {
		helperCalled = true
		if !autoInstall {
			t.Fatalf("autoInstall = false, want true")
		}
		if len(hostsCfg.Entries) == 0 {
			t.Fatalf("hosts entries were not passed to helper")
		}
		return privilege.PrepareHostsResult{
			Cert: certstore.TrustResult{
				StoreScope:     certCfg.StoreScope,
				AlreadyTrusted: true,
				ViaHelper:      true,
			},
			CertTrusted: true,
			Entries:     len(hostsCfg.Entries),
		}, nil
	}
	shouldUseHostHelper = func() bool { return true }
	defer func() {
		prepareHostsElevated = oldPrepare
		shouldUseHostHelper = oldShouldUseHelper
	}()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if !helperCalled {
		t.Fatalf("elevated helper was not called")
	}
	if !status.CertInstalled {
		t.Fatalf("cert should be reported as installed")
	}
	if !hasSystemChange(status.SystemChanges, "root_ca", "check", "already_trusted") {
		t.Fatalf("root CA helper change missing: %#v", status.SystemChanges)
	}
	if !hasSystemChange(status.SystemChanges, "hosts", "apply", "ok") {
		t.Fatalf("hosts helper apply change missing: %#v", status.SystemChanges)
	}
	foundHelperDetail := false
	for _, change := range status.SystemChanges {
		if strings.Contains(change.Detail, "helper=elevated") {
			foundHelperDetail = true
			break
		}
	}
	if !foundHelperDetail {
		t.Fatalf("helper detail missing: %#v", status.SystemChanges)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestEngineHostsModeFallsBackToElevatedHelperOnAccessDenied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only elevated helper fallback")
	}
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Hosts.HTTPListenAddr = "127.0.0.1:0"
	cfg.Hosts.HTTPSListenAddr = "127.0.0.1:0"
	cfg.Cert.Dir = t.TempDir()
	cfg.Runtime.RollbackPath = config.DefaultRollbackPath()

	var helperCalled bool
	restoreFns := replaceHostsHooks(
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("preflightHosts should not run after certificate access denied")
			return nil
		},
		func(ctx context.Context, cfg hosts.Config) error {
			t.Fatalf("applyHosts should not run after certificate access denied")
			return nil
		},
		func(ctx context.Context, path string) error { return nil },
		func(ctx context.Context, cfg certstore.Config) (bool, error) {
			return false, nil
		},
		func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
			return certstore.TrustResult{}, errors.New("Access is denied.")
		},
		func(path string) bool { return true },
	)
	defer restoreFns()
	oldPrepare := prepareHostsElevated
	prepareHostsElevated = func(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (privilege.PrepareHostsResult, error) {
		helperCalled = true
		return privilege.PrepareHostsResult{
			Cert: certstore.TrustResult{
				StoreScope: cfg.Cert.StoreScope,
				Installed:  true,
				Changed:    true,
				ViaHelper:  true,
			},
			CertTrusted: true,
			Entries:     len(hostsCfg.Entries),
		}, nil
	}
	defer func() {
		prepareHostsElevated = oldPrepare
	}()
	restoreProbe := replaceStartupProbeHook(func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return nil
	})
	defer restoreProbe()

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
	if !helperCalled {
		t.Fatalf("elevated helper fallback was not called")
	}
	if !hasSystemChange(status.SystemChanges, "root_ca", "install", "ok") {
		t.Fatalf("root CA helper install change missing: %#v", status.SystemChanges)
	}
	foundHelperDetail := false
	for _, change := range status.SystemChanges {
		if strings.Contains(change.Detail, "helper=elevated") {
			foundHelperDetail = true
			break
		}
	}
	if !foundHelperDetail {
		t.Fatalf("helper detail missing: %#v", status.SystemChanges)
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

func replaceHostsHooks(preflight func(context.Context, hosts.Config) error, apply func(context.Context, hosts.Config) error, restore func(context.Context, string) error, certCheck func(context.Context, certstore.Config) (bool, error), certTrust func(context.Context, certstore.Config) (certstore.TrustResult, error), has func(string) bool) func() {
	oldPreflight := preflightHosts
	oldApply := applyHosts
	oldRestore := restoreHosts
	oldCertCheck := isCertInstalled
	oldCertTrust := ensureCertTrusted
	oldHas := hasRollbackState
	oldShouldUseHelper := shouldUseHostHelper
	preflightHosts = preflight
	applyHosts = apply
	restoreHosts = restore
	isCertInstalled = certCheck
	ensureCertTrusted = certTrust
	hasRollbackState = has
	shouldUseHostHelper = func() bool { return false }
	return func() {
		preflightHosts = oldPreflight
		applyHosts = oldApply
		restoreHosts = oldRestore
		isCertInstalled = oldCertCheck
		ensureCertTrusted = oldCertTrust
		hasRollbackState = oldHas
		shouldUseHostHelper = oldShouldUseHelper
	}
}

func replaceSystemDNSHooks(preflight func(context.Context, systemdns.Config) (systemdns.Result, error), apply func(context.Context, systemdns.Config) (systemdns.Result, error), restore func(context.Context, string) error) func() {
	oldPreflight := preflightSystemDNS
	oldApply := applySystemDNS
	oldRestore := restoreSystemDNS
	preflightSystemDNS = preflight
	applySystemDNS = apply
	restoreSystemDNS = restore
	return func() {
		preflightSystemDNS = oldPreflight
		applySystemDNS = oldApply
		restoreSystemDNS = oldRestore
	}
}

func replaceDNSInterceptHook(hook func(dnsintercept.Config, *rules.Matcher, dnsintercept.Forwarder) (dnsInterceptServer, error)) func() {
	oldNew := newDNSIntercept
	newDNSIntercept = hook
	return func() {
		newDNSIntercept = oldNew
	}
}

func hasSystemChange(changes []SystemChange, component, action, status string) bool {
	for _, change := range changes {
		if change.Component == component && change.Action == action && change.Status == status {
			return true
		}
	}
	return false
}

func replaceStartupProbeHook(probe func(context.Context, *upstream.DirectDialer, []upstream.ProbeTarget) []upstream.ProbeResult) func() {
	oldProbe := runStartupProbes
	runStartupProbes = probe
	return func() {
		runStartupProbes = oldProbe
	}
}

type fakeDNSInterceptServer struct {
	status  dnsintercept.Status
	onStart func()
	onStop  func()
}

func (s *fakeDNSInterceptServer) Start() error {
	if s.onStart != nil {
		s.onStart()
	}
	return nil
}

func (s *fakeDNSInterceptServer) Stop(context.Context) error {
	if s.onStop != nil {
		s.onStop()
	}
	return nil
}

func (s *fakeDNSInterceptServer) Status() dnsintercept.Status {
	return s.status
}

func startEngineTestDNSServer(t *testing.T) (string, func()) {
	t.Helper()
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		if len(req.Question) > 0 && req.Question[0].Qtype == dns.TypeA {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("192.0.2.88"),
			})
		}
		_ = w.WriteMsg(resp)
	})
	server := &dns.Server{Net: "udp", Handler: handler}
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.PacketConn = conn
	go func() { _ = server.ActivateAndServe() }()
	return conn.LocalAddr().String(), func() { _ = server.Shutdown() }
}

func exchangeEngineDNS(t *testing.T, addr, host string, qtype uint16) *dns.Msg {
	t.Helper()
	msg := new(dns.Msg)
	msg.SetQuestion(host, qtype)
	client := &dns.Client{Net: "udp", Timeout: time.Second}
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, _, err := client.Exchange(msg.Copy(), addr)
		if err == nil {
			return resp
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("exchange DNS %s: %v", addr, lastErr)
	return nil
}

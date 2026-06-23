package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/dnsintercept"
	"github.com/gofurry/go-steam-core/internal/engine"
	"github.com/gofurry/go-steam-core/internal/pageenhance"
	"github.com/gofurry/go-steam-core/internal/provider"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

func TestRestoreNoRollbackState(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"restore", "--rollback", filepath.Join(t.TempDir(), "rollback.json")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "not modified" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRestoreRollbackDispatchesSystemDNS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.json")
	if err := os.WriteFile(path, []byte(`{"kind":"system_dns","mode":"dns"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHosts := restoreHostsRollback
	oldDNS := restoreSystemDNSRollback
	oldProxy := restoreSystemProxyRollback
	defer func() {
		restoreHostsRollback = oldHosts
		restoreSystemDNSRollback = oldDNS
		restoreSystemProxyRollback = oldProxy
	}()
	var restoredPath string
	restoreHostsRollback = func(ctx context.Context, path string) error {
		t.Fatalf("hosts restore should not be called")
		return nil
	}
	restoreSystemDNSRollback = func(ctx context.Context, path string) error {
		restoredPath = path
		return nil
	}
	restoreSystemProxyRollback = func(ctx context.Context, path string) error {
		t.Fatalf("system proxy restore should not be called")
		return nil
	}
	if err := restoreRollback(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if restoredPath != path {
		t.Fatalf("restored path = %q, want %q", restoredPath, path)
	}
}

func TestPrintStartupProbes(t *testing.T) {
	var stdout bytes.Buffer
	printStartupProbes(&stdout, []upstream.ProbeResult{
		{ProviderID: provider.IDSteam, Host: "steamcommunity.com", OK: true, Stage: "http"},
		{ProviderID: provider.IDSteam, Host: "store.steampowered.com", Target: "cdn-a.akamaihd.net", Stage: "tcp", Error: "tcp 1.2.3.4:443 failed"},
	})
	got := stdout.String()
	for _, want := range []string{
		"startup_probes: ok=1 failed=1",
		"startup_probe_failed: provider=steam host=store.steampowered.com target=cdn-a.akamaihd.net stage=tcp error=tcp 1.2.3.4:443 failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestRunStartRejectsLegacyNonSteamFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"start", "--non-steam", "direct"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "use --non-target") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPrintProviders(t *testing.T) {
	var stdout bytes.Buffer
	printProviders(&stdout, engine.Status{Providers: []provider.Summary{{
		ID:               provider.IDGitHub,
		Status:           provider.StatusExperimental,
		RuleSetName:      provider.GitHubRuleSetName,
		RuleSetVersion:   provider.GitHubRuleSetVersion,
		OutboundProfiles: 0,
		ProbeTargets:     3,
	}}})
	got := strings.TrimSpace(stdout.String())
	want := "provider: id=github status=experimental rule_set=github-web@2026.06.23 probes=3"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunAppHostRejectsUnsupportedSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runAppHost([]string{"bogus"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported apphost subcommand") {
		t.Fatalf("runAppHost() error = %v", err)
	}
}

func TestPrintRuleSet(t *testing.T) {
	var stdout bytes.Buffer
	printRuleSet(&stdout, engine.Status{RuleSetName: "steam-web", RuleSetVersion: "2026.06.23"})
	if got, want := strings.TrimSpace(stdout.String()), "rule_set: steam-web@2026.06.23"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintDNSIntercept(t *testing.T) {
	var stdout bytes.Buffer
	printDNSIntercept(&stdout, engine.Status{DNSIntercept: &dnsintercept.Status{
		Strategy:         "manual",
		ListenAddr:       "127.0.0.1:15353",
		SystemDNS:        false,
		TargetQueries:    2,
		ForwardedQueries: 3,
		CacheHits:        4,
		BlockedQueries:   1,
		ErrorQueries:     0,
	}})
	got := strings.TrimSpace(stdout.String())
	want := "dns_intercept: strategy=manual listen=127.0.0.1:15353 system_dns=false target=2 forwarded=3 cache_hits=4 blocked=1 errors=0"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintPageEnhance(t *testing.T) {
	var stdout bytes.Buffer
	printPageEnhance(&stdout, engine.Status{PageEnhance: &pageenhance.Status{
		Enabled:    true,
		OnError:    pageenhance.OnErrorPassThrough,
		Transforms: 2,
		Assets:     1,
		Applied:    3,
		Skipped:    4,
		Errors:     5,
	}})
	got := strings.TrimSpace(stdout.String())
	want := "page_enhance: enabled=true on_error=pass_through transforms=2 assets=1 applied=3 skipped=4 errors=5"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintSystemChanges(t *testing.T) {
	var stdout bytes.Buffer
	printSystemChanges(&stdout, []engine.SystemChange{{
		Component: "root_ca",
		Action:    "install",
		Status:    "ok",
		Detail:    "installed",
	}})
	got := strings.TrimSpace(stdout.String())
	want := "system_change: component=root_ca action=install status=ok detail=installed"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

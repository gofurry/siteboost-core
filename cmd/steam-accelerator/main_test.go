package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/engine"
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
	printRuleSet(&stdout, engine.Status{RuleSetName: "steam-web", RuleSetVersion: "2026.06.22"})
	if got, want := strings.TrimSpace(stdout.String()), "rule_set: steam-web@2026.06.22"; got != want {
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

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/engine"
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
		{Host: "steamcommunity.com", OK: true, Stage: "http"},
		{Host: "store.steampowered.com", Target: "cdn-a.akamaihd.net", Stage: "tcp", Error: "tcp 1.2.3.4:443 failed"},
	})
	got := stdout.String()
	for _, want := range []string{
		"startup_probes: ok=1 failed=1",
		"startup_probe_failed: host=store.steampowered.com target=cdn-a.akamaihd.net stage=tcp error=tcp 1.2.3.4:443 failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestPrintRuleSet(t *testing.T) {
	var stdout bytes.Buffer
	printRuleSet(&stdout, engine.Status{RuleSetName: "steam-web", RuleSetVersion: "2026.06.22"})
	if got, want := strings.TrimSpace(stdout.String()), "rule_set: steam-web@2026.06.22"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

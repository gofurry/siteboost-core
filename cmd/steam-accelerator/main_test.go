package main

import (
	"bytes"
	"path/filepath"
	"reflect"
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

func TestRunStartRelaunchesHostsModeWhenHelperRequired(t *testing.T) {
	oldShouldRelaunch := shouldRelaunchHostsStart
	oldRelaunch := relaunchHostsStart
	defer func() {
		shouldRelaunchHostsStart = oldShouldRelaunch
		relaunchHostsStart = oldRelaunch
	}()

	shouldRelaunchHostsStart = func() bool { return true }
	var gotArgs []string
	relaunchHostsStart = func(args []string) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	statePath := filepath.Join(t.TempDir(), "runtime.json")
	var stdout, stderr bytes.Buffer
	err := runStart([]string{"--mode", "hosts", "--state", statePath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStart returned error: %v; stderr=%s", err, stderr.String())
	}
	wantArgs := []string{"start", "--mode", "hosts", "--state", statePath, "--elevated-child"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("relaunch args = %#v, want %#v", gotArgs, wantArgs)
	}
	if !strings.Contains(stdout.String(), "relaunching hosts mode with administrator privileges") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunStartElevatedChildRequiresAdministratorToken(t *testing.T) {
	oldShouldRelaunch := shouldRelaunchHostsStart
	oldRelaunch := relaunchHostsStart
	defer func() {
		shouldRelaunchHostsStart = oldShouldRelaunch
		relaunchHostsStart = oldRelaunch
	}()

	shouldRelaunchHostsStart = func() bool { return true }
	relaunchHostsStart = func(args []string) error {
		t.Fatalf("unexpected relaunch with args %#v", args)
		return nil
	}

	var stdout, stderr bytes.Buffer
	err := runStart([]string{"--mode", "hosts", "--elevated-child"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("runStart returned nil error")
	}
	if !strings.Contains(err.Error(), "administrator token") {
		t.Fatalf("error = %v", err)
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

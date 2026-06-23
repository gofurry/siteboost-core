package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	steamcore "github.com/gofurry/go-steam-core"
	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/engine"
	"github.com/gofurry/go-steam-core/internal/hosts"
	"github.com/gofurry/go-steam-core/internal/privilege"
	runtimecontrol "github.com/gofurry/go-steam-core/internal/runtime"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	if args[0] == "--version" || args[0] == "-version" || args[0] == "version" {
		fmt.Fprintf(stdout, "%s %s (%s)\n", steamcore.ProjectName, steamcore.Version, steamcore.ModulePath)
		return 0
	}

	var err error
	switch args[0] {
	case "start":
		err = runStart(args[1:], stdout, stderr)
	case "status":
		err = runStatus(args[1:], stdout, stderr)
	case "stop":
		err = runStop(args[1:], stdout, stderr)
	case "restore":
		err = runRestore(args[1:], stdout, stderr)
	case "cert":
		err = runCert(args[1:], stdout, stderr)
	case "apphost":
		err = runAppHost(args[1:], stdout, stderr)
	case "__helper":
		err = privilege.RunHelper(args[1:], stdout, stderr)
	case "__apphost-service":
		err = privilege.RunAppHostService(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func runStart(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to YAML config")
	mode := fs.String("mode", "", "acceleration mode: proxy-only, pac, system, or hosts")
	listen := fs.String("listen", "", "proxy listen address")
	pacListen := fs.String("pac-listen", "", "PAC server listen address")
	hostsHTTP := fs.String("hosts-http", "", "hosts mode HTTP listen address")
	hostsHTTPS := fs.String("hosts-https", "", "hosts mode HTTPS listen address")
	nonTarget := fs.String("non-target", "", "non-target behavior: reject or direct")
	allowLAN := fs.Bool("allow-lan", false, "allow non-loopback proxy listen address")
	statePath := fs.String("state", "", "runtime state file path")
	controlAddr := fs.String("control", "", "control listen address")
	if hasLegacyFlag(args, "non-steam") {
		return fmt.Errorf("--non-steam was removed in v0.7; use --non-target")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	visited := visitedFlags(fs)

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	applyStartOverrides(&cfg, visited, *mode, *listen, *pacListen, *hostsHTTP, *hostsHTTPS, *nonTarget, *allowLAN, *statePath, *controlAddr)
	if err := cfg.Validate(); err != nil {
		return err
	}

	if running, err := runningFromState(cfg.Runtime.StatePath); err == nil && running {
		return fmt.Errorf("steam-accelerator is already running")
	}

	logger := slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	eng, err := engine.New(cfg, logger)
	if err != nil {
		return err
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		return err
	}

	token, err := runtimecontrol.GenerateToken()
	if err != nil {
		return fmt.Errorf("generate control token: %w", err)
	}
	control, err := runtimecontrol.NewControlServer(cfg.Runtime.ControlAddr, token, func() any {
		return eng.Status()
	}, cancel)
	if err != nil {
		return err
	}
	if err := control.Start(); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), cfg.Proxy.ShutdownTimeout.Std())
		defer stopCancel()
		_ = eng.Stop(stopCtx)
		return err
	}

	status := eng.Status()
	state := runtimecontrol.State{
		PID:        os.Getpid(),
		Mode:       cfg.Mode,
		ProxyAddr:  status.ListenAddr,
		PACURL:     status.PACURL,
		HostsHTTP:  status.HostsHTTP,
		HostsHTTPS: status.HostsHTTPS,
		ControlURL: control.URL(),
		Token:      token,
		StartedAt:  status.StartedAt,
	}
	if err := runtimecontrol.WriteState(cfg.Runtime.StatePath, state); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), cfg.Proxy.ShutdownTimeout.Std())
		defer stopCancel()
		_ = control.Stop(stopCtx)
		_ = eng.Stop(stopCtx)
		return err
	}
	defer runtimecontrol.RemoveState(cfg.Runtime.StatePath)

	fmt.Fprintf(stdout, "%s started\n", steamcore.ProjectName)
	if status.ListenAddr != "" {
		fmt.Fprintf(stdout, "proxy: %s\n", status.ListenAddr)
	}
	if status.PACURL != "" {
		fmt.Fprintf(stdout, "pac_url: %s\n", status.PACURL)
	}
	if status.HostsHTTP != "" {
		fmt.Fprintf(stdout, "hosts_http: %s\n", status.HostsHTTP)
	}
	if status.HostsHTTPS != "" {
		fmt.Fprintf(stdout, "hosts_https: %s\n", status.HostsHTTPS)
	}
	if status.ResolverMode != "" {
		fmt.Fprintf(stdout, "resolver: %s\n", status.ResolverMode)
	}
	if len(status.ResolverServers) > 0 {
		fmt.Fprintf(stdout, "resolver_servers: %s\n", strings.Join(status.ResolverServers, ","))
	}
	if status.UpstreamProfiles > 0 {
		fmt.Fprintf(stdout, "upstream_profiles: %d\n", status.UpstreamProfiles)
	}
	printProviders(stdout, status)
	printRuleSet(stdout, status)
	printSystemChanges(stdout, status.SystemChanges)
	printStartupProbes(stdout, status.StartupProbes)
	fmt.Fprintf(stdout, "state: %s\n", cfg.Runtime.StatePath)
	<-ctx.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), cfg.Proxy.ShutdownTimeout.Std())
	defer stopCancel()
	controlErr := control.Stop(stopCtx)
	engineErr := eng.Stop(stopCtx)
	if controlErr != nil {
		return controlErr
	}
	return engineErr
}

func runStatus(args []string, stdout, stderr io.Writer) error {
	statePath, err := statePathFromFlags("status", args, stderr)
	if err != nil {
		return err
	}
	state, err := runtimecontrol.ReadState(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stdout, "not running")
			return nil
		}
		return err
	}
	status, err := queryStatus(state)
	if err != nil {
		_ = runtimecontrol.RemoveState(statePath)
		fmt.Fprintln(stdout, "not running (stale state removed)")
		return nil
	}

	fmt.Fprintf(stdout, "running: %v\n", status.Running)
	fmt.Fprintf(stdout, "mode: %s\n", status.Mode)
	fmt.Fprintf(stdout, "proxy: %s\n", status.ListenAddr)
	if status.PACURL != "" {
		fmt.Fprintf(stdout, "pac_url: %s\n", status.PACURL)
	}
	if status.HostsHTTP != "" {
		fmt.Fprintf(stdout, "hosts_http: %s\n", status.HostsHTTP)
	}
	if status.HostsHTTPS != "" {
		fmt.Fprintf(stdout, "hosts_https: %s\n", status.HostsHTTPS)
	}
	if status.ResolverMode != "" {
		fmt.Fprintf(stdout, "resolver: %s\n", status.ResolverMode)
	}
	if len(status.ResolverServers) > 0 {
		fmt.Fprintf(stdout, "resolver_servers: %s\n", strings.Join(status.ResolverServers, ","))
	}
	if status.UpstreamProfiles > 0 {
		fmt.Fprintf(stdout, "upstream_profiles: %d\n", status.UpstreamProfiles)
	}
	printProviders(stdout, status)
	printRuleSet(stdout, status)
	printSystemChanges(stdout, status.SystemChanges)
	printStartupProbes(stdout, status.StartupProbes)
	fmt.Fprintf(stdout, "rollback: %v\n", status.Rollback)
	if status.Mode == config.ModeHosts {
		fmt.Fprintf(stdout, "cert_installed: %v\n", status.CertInstalled)
	}
	fmt.Fprintf(stdout, "active_conns: %d\n", status.ActiveConns)
	if !status.StartedAt.IsZero() {
		fmt.Fprintf(stdout, "started_at: %s\n", status.StartedAt.Format(time.RFC3339))
	}
	if status.LastError != "" {
		fmt.Fprintf(stdout, "last_error: %s\n", status.LastError)
	}
	return nil
}

func runStop(args []string, stdout, stderr io.Writer) error {
	statePath, err := statePathFromFlags("stop", args, stderr)
	if err != nil {
		return err
	}
	state, err := runtimecontrol.ReadState(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stdout, "not running")
			return nil
		}
		return err
	}

	if err := postStop(state); err != nil {
		_ = runtimecontrol.RemoveState(statePath)
		fmt.Fprintln(stdout, "not running (stale state removed)")
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := queryStatus(state); err != nil {
			_ = runtimecontrol.RemoveState(statePath)
			fmt.Fprintln(stdout, "stopped")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = runtimecontrol.RemoveState(statePath)
	fmt.Fprintln(stdout, "stop requested")
	return nil
}

func runRestore(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to YAML config")
	rollbackPath := fs.String("rollback", "", "rollback state file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*rollbackPath) != "" {
		cfg.Runtime.RollbackPath = *rollbackPath
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Runtime.StopTimeout.Std())
	defer cancel()
	err = restoreRollback(ctx, cfg.Runtime.RollbackPath)
	if errors.Is(err, systemproxy.ErrNoState) || errors.Is(err, hosts.ErrNoState) {
		fmt.Fprintln(stdout, "not modified")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "restored")
	return nil
}

func runCert(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("cert subcommand is required")
	}
	switch args[0] {
	case "install", "uninstall":
	default:
		printUsage(stderr)
		return fmt.Errorf("unsupported cert subcommand %q", args[0])
	}

	fs := flag.NewFlagSet("cert "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to YAML config")
	certDir := fs.String("cert-dir", "", "certificate directory")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*certDir) != "" {
		cfg.Cert.Dir = *certDir
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Runtime.StopTimeout.Std())
	defer cancel()
	certCfg := certstore.ConfigFromApp(cfg)
	switch args[0] {
	case "install":
		if err := privilege.InstallCert(ctx, certCfg); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "installed")
	case "uninstall":
		if err := privilege.UninstallCert(ctx, certCfg); err != nil {
			if errors.Is(err, certstore.ErrNoCA) {
				fmt.Fprintln(stdout, "not modified")
				return nil
			}
			return err
		}
		fmt.Fprintln(stdout, "uninstalled")
	}
	return nil
}

func runAppHost(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("apphost subcommand is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch args[0] {
	case "install":
		if err := privilege.InstallAppHostService(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "apphost installed and started")
	case "uninstall":
		if err := privilege.UninstallAppHostService(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "apphost uninstalled")
	case "start":
		if err := privilege.StartAppHostService(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "apphost started")
	case "stop":
		if err := privilege.StopAppHostService(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "apphost stopped")
	case "status":
		status, err := privilege.AppHostServiceStatus(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "apphost: %s\n", status)
	case "run":
		runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return privilege.RunAppHostConsole(runCtx, stdout)
	default:
		printUsage(stderr)
		return fmt.Errorf("unsupported apphost subcommand %q", args[0])
	}
	return nil
}

func statePathFromFlags(name string, args []string, stderr io.Writer) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to YAML config")
	statePath := fs.String("state", "", "runtime state file path")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(*statePath) != "" {
		cfg.Runtime.StatePath = *statePath
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	return cfg.Runtime.StatePath, nil
}

func queryStatus(state runtimecontrol.State) (engine.Status, error) {
	var status engine.Status
	req, err := http.NewRequest(http.MethodGet, state.ControlURL+"/status", nil)
	if err != nil {
		return status, err
	}
	req.Header.Set("Authorization", "Bearer "+state.Token)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("control status returned %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return status, err
	}
	return status, nil
}

func postStop(state runtimecontrol.State) error {
	req, err := http.NewRequest(http.MethodPost, state.ControlURL+"/stop", bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+state.Token)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("control stop returned %s", resp.Status)
	}
	return nil
}

func runningFromState(path string) (bool, error) {
	state, err := runtimecontrol.ReadState(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	_, err = queryStatus(state)
	if err != nil {
		_ = runtimecontrol.RemoveState(path)
		return false, nil
	}
	return true, nil
}

func applyStartOverrides(cfg *config.Config, visited map[string]bool, mode, listen, pacListen, hostsHTTP, hostsHTTPS, nonTarget string, allowLAN bool, statePath, controlAddr string) {
	if visited["mode"] {
		cfg.Mode = mode
	}
	if visited["listen"] {
		cfg.Proxy.ListenAddr = listen
	}
	if visited["pac-listen"] {
		cfg.PAC.ListenAddr = pacListen
	}
	if visited["hosts-http"] {
		cfg.Hosts.HTTPListenAddr = hostsHTTP
	}
	if visited["hosts-https"] {
		cfg.Hosts.HTTPSListenAddr = hostsHTTPS
	}
	if visited["non-target"] {
		cfg.Proxy.NonTargetBehavior = nonTarget
	}
	if visited["allow-lan"] {
		cfg.Proxy.AllowLAN = allowLAN
	}
	if visited["state"] {
		cfg.Runtime.StatePath = statePath
	}
	if visited["control"] {
		cfg.Runtime.ControlAddr = controlAddr
	}
}

func restoreRollback(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return systemproxy.ErrNoState
		}
		return err
	}
	var meta struct {
		Kind string `json:"kind"`
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parse rollback state: %w", err)
	}
	if meta.Kind == "hosts" || meta.Mode == config.ModeHosts {
		return privilege.RestoreHosts(ctx, path)
	}
	return systemproxy.Restore(ctx, path)
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func hasLegacyFlag(args []string, name string) bool {
	prefix := "--" + name
	shortPrefix := "-" + name
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") || arg == shortPrefix || strings.HasPrefix(arg, shortPrefix+"=") {
			return true
		}
	}
	return false
}

func printProviders(w io.Writer, status engine.Status) {
	for _, p := range status.Providers {
		if p.ID == "" {
			continue
		}
		fmt.Fprintf(w, "provider: id=%s status=%s", p.ID, p.Status)
		if p.RuleSetName != "" {
			fmt.Fprintf(w, " rule_set=%s", p.RuleSetName)
			if p.RuleSetVersion != "" {
				fmt.Fprintf(w, "@%s", p.RuleSetVersion)
			}
		}
		if p.OutboundProfiles > 0 {
			fmt.Fprintf(w, " profiles=%d", p.OutboundProfiles)
		}
		if p.ProbeTargets > 0 {
			fmt.Fprintf(w, " probes=%d", p.ProbeTargets)
		}
		fmt.Fprintln(w)
	}
}

func printStartupProbes(w io.Writer, probes []upstream.ProbeResult) {
	if len(probes) == 0 {
		return
	}
	okCount := 0
	for _, probe := range probes {
		if probe.OK {
			okCount++
		}
	}
	failed := len(probes) - okCount
	fmt.Fprintf(w, "startup_probes: ok=%d failed=%d\n", okCount, failed)
	for _, probe := range probes {
		if probe.OK {
			continue
		}
		fmt.Fprintf(w, "startup_probe_failed:")
		if probe.ProviderID != "" {
			fmt.Fprintf(w, " provider=%s", probe.ProviderID)
		}
		fmt.Fprintf(w, " host=%s", probe.Host)
		if probe.Target != "" {
			fmt.Fprintf(w, " target=%s", probe.Target)
		}
		if probe.Stage != "" {
			fmt.Fprintf(w, " stage=%s", probe.Stage)
		}
		if probe.Error != "" {
			fmt.Fprintf(w, " error=%s", truncateProbeError(probe.Error))
		}
		fmt.Fprintln(w)
	}
}

func printRuleSet(w io.Writer, status engine.Status) {
	if status.RuleSetName == "" {
		return
	}
	if status.RuleSetVersion != "" {
		fmt.Fprintf(w, "rule_set: %s@%s\n", status.RuleSetName, status.RuleSetVersion)
		return
	}
	fmt.Fprintf(w, "rule_set: %s\n", status.RuleSetName)
}

func printSystemChanges(w io.Writer, changes []engine.SystemChange) {
	for _, change := range changes {
		if change.Component == "" {
			continue
		}
		fmt.Fprintf(w, "system_change: component=%s action=%s status=%s", change.Component, change.Action, change.Status)
		if change.Detail != "" {
			fmt.Fprintf(w, " detail=%s", change.Detail)
		}
		fmt.Fprintln(w)
	}
}

func truncateProbeError(err string) string {
	err = strings.TrimSpace(err)
	if len(err) <= 320 {
		return err
	}
	return err[:320] + "..."
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s %s

Usage:
  steam-accelerator --version
  steam-accelerator start [--config path] [--mode proxy-only|pac|system|hosts] [--listen 127.0.0.1:26501] [--pac-listen 127.0.0.1:26502] [--hosts-http 127.0.0.1:80] [--hosts-https 127.0.0.1:443] [--non-target reject|direct]
  steam-accelerator status [--config path] [--state path]
  steam-accelerator stop [--config path] [--state path]
  steam-accelerator restore [--config path] [--rollback path]
  steam-accelerator cert install [--config path] [--cert-dir path]
  steam-accelerator cert uninstall [--config path] [--cert-dir path]
  steam-accelerator apphost install|start|stop|status|uninstall|run
`, steamcore.ProjectName, steamcore.Version)
}

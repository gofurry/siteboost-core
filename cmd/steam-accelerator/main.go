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
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/engine"
	runtimecontrol "github.com/gofurry/go-steam-core/internal/runtime"
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
	mode := fs.String("mode", "", "acceleration mode: proxy-only")
	listen := fs.String("listen", "", "proxy listen address")
	nonSteam := fs.String("non-steam", "", "non-Steam behavior: reject or direct")
	allowLAN := fs.Bool("allow-lan", false, "allow non-loopback proxy listen address")
	statePath := fs.String("state", "", "runtime state file path")
	controlAddr := fs.String("control", "", "control listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	visited := visitedFlags(fs)

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	applyStartOverrides(&cfg, visited, *mode, *listen, *nonSteam, *allowLAN, *statePath, *controlAddr)
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

	fmt.Fprintf(stdout, "%s started\nproxy: %s\nstate: %s\n", steamcore.ProjectName, status.ListenAddr, cfg.Runtime.StatePath)
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

func applyStartOverrides(cfg *config.Config, visited map[string]bool, mode, listen, nonSteam string, allowLAN bool, statePath, controlAddr string) {
	if visited["mode"] {
		cfg.Mode = mode
	}
	if visited["listen"] {
		cfg.Proxy.ListenAddr = listen
	}
	if visited["non-steam"] {
		cfg.Proxy.NonSteamBehavior = nonSteam
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

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s %s

Usage:
  steam-accelerator --version
  steam-accelerator start [--config path] [--mode proxy-only] [--listen 127.0.0.1:26501] [--non-steam reject|direct]
  steam-accelerator status [--config path] [--state path]
  steam-accelerator stop [--config path] [--state path]
`, steamcore.ProjectName, steamcore.Version)
}

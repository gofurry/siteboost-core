package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/proxy"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

type Status struct {
	Running     bool      `json:"running"`
	Mode        string    `json:"mode"`
	ListenAddr  string    `json:"listen_addr"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	RuleCount   int       `json:"rule_count"`
	ActiveConns int64     `json:"active_conns"`
}

type Engine struct {
	mu     sync.Mutex
	cfg    config.Config
	logger *slog.Logger

	proxy     *proxy.Server
	startedAt time.Time
	lastErr   error
	ruleCount int
	running   bool
}

func New(cfg config.Config, logger *slog.Logger) (*Engine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Engine{cfg: cfg, logger: logger}, nil
}

func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.mu.Unlock()

	groups := []rules.RuleGroup(nil)
	if e.cfg.Rules.EnableDefaultSteamRules {
		groups = rules.DefaultSteamRules
	}
	matcher, err := rules.NewMatcher(groups, e.cfg.Rules.CustomDomains)
	if err != nil {
		return fmt.Errorf("build rules matcher: %w", err)
	}

	dialer := upstream.NewDirectDialer(e.cfg.Proxy.DialTimeout.Std())
	proxyServer := proxy.New(proxy.ConfigFromApp(e.cfg), matcher, dialer, e.logger)
	if err := proxyServer.Start(); err != nil {
		return err
	}

	e.mu.Lock()
	e.proxy = proxyServer
	e.startedAt = time.Now()
	e.lastErr = nil
	e.ruleCount = matcher.RuleCount()
	e.running = true
	e.mu.Unlock()

	go func() {
		<-ctx.Done()
		stopCtx, cancel := context.WithTimeout(context.Background(), e.cfg.Proxy.ShutdownTimeout.Std())
		defer cancel()
		_ = e.Stop(stopCtx)
	}()

	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	proxyServer := e.proxy
	e.proxy = nil
	e.running = false
	e.mu.Unlock()

	if proxyServer == nil {
		return nil
	}
	err := proxyServer.Stop(ctx)

	e.mu.Lock()
	e.lastErr = err
	e.mu.Unlock()

	return err
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()

	status := Status{
		Running:   e.running,
		Mode:      e.cfg.Mode,
		StartedAt: e.startedAt,
		RuleCount: e.ruleCount,
	}
	if e.proxy != nil {
		status.ListenAddr = e.proxy.Addr()
		status.ActiveConns = e.proxy.ActiveConns()
	}
	if e.lastErr != nil {
		status.LastError = e.lastErr.Error()
	}
	return status
}

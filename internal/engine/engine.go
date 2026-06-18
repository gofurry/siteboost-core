package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/pac"
	"github.com/gofurry/go-steam-core/internal/proxy"
	"github.com/gofurry/go-steam-core/internal/resolver"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

var (
	applySystemProxy   = systemproxy.Apply
	restoreSystemProxy = systemproxy.Restore
	hasRollbackState   = systemproxy.HasState
)

type Status struct {
	Running     bool      `json:"running"`
	Mode        string    `json:"mode"`
	ListenAddr  string    `json:"listen_addr"`
	PACURL      string    `json:"pac_url,omitempty"`
	Rollback    bool      `json:"rollback"`
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
	pac       *pac.Server
	pacURL    string
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

	dnsResolver, err := resolver.New(resolver.ConfigFromApp(e.cfg))
	if err != nil {
		return fmt.Errorf("build resolver: %w", err)
	}
	dialer, err := upstream.NewDialer(upstream.ConfigFromApp(e.cfg), dnsResolver)
	if err != nil {
		return fmt.Errorf("build upstream dialer: %w", err)
	}
	proxyServer := proxy.New(proxy.ConfigFromApp(e.cfg), matcher, dialer, e.logger)
	if err := proxyServer.Start(); err != nil {
		return err
	}
	var pacServer *pac.Server
	var pacURL string
	if e.cfg.Mode == config.ModePAC {
		pacServer, err = pac.New(pac.ConfigFromApp(e.cfg, proxyServer.Addr()), matcher.Rules(), e.logger)
		if err != nil {
			_ = proxyServer.Stop(context.Background())
			return fmt.Errorf("build PAC server: %w", err)
		}
		if err := pacServer.Start(); err != nil {
			_ = proxyServer.Stop(context.Background())
			return err
		}
		pacURL = pacServer.URL()
	}
	if e.cfg.Mode == config.ModePAC || e.cfg.Mode == config.ModeSystem {
		sysCfg := systemproxy.ConfigFromApp(e.cfg, proxyServer.Addr(), pacURL)
		if err := applySystemProxy(ctx, sysCfg); err != nil {
			if pacServer != nil {
				_ = pacServer.Stop(context.Background())
			}
			_ = proxyServer.Stop(context.Background())
			return fmt.Errorf("apply system proxy: %w", err)
		}
	}

	e.mu.Lock()
	e.proxy = proxyServer
	e.pac = pacServer
	e.pacURL = pacURL
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
	pacServer := e.pac
	e.proxy = nil
	e.pac = nil
	e.pacURL = ""
	e.running = false
	e.mu.Unlock()

	if proxyServer == nil {
		return nil
	}
	var err error
	if e.cfg.Mode == config.ModePAC || e.cfg.Mode == config.ModeSystem {
		restoreErr := restoreSystemProxy(ctx, e.cfg.Runtime.RollbackPath)
		if restoreErr != nil && !errors.Is(restoreErr, systemproxy.ErrNoState) {
			err = restoreErr
		}
	}
	if pacServer != nil {
		if stopErr := pacServer.Stop(ctx); err == nil {
			err = stopErr
		}
	}
	if stopErr := proxyServer.Stop(ctx); err == nil {
		err = stopErr
	}

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
		PACURL:    e.pacURL,
		Rollback:  hasRollbackState(e.cfg.Runtime.RollbackPath),
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

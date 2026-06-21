package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/hosts"
	"github.com/gofurry/go-steam-core/internal/pac"
	"github.com/gofurry/go-steam-core/internal/proxy"
	"github.com/gofurry/go-steam-core/internal/resolver"
	"github.com/gofurry/go-steam-core/internal/reverse"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

var (
	applySystemProxy   = systemproxy.Apply
	restoreSystemProxy = systemproxy.Restore
	hasRollbackState   = systemproxy.HasState
	preflightHosts     = hosts.Preflight
	applyHosts         = hosts.Apply
	restoreHosts       = hosts.Restore
	isCertInstalled    = func(ctx context.Context, cfg certstore.Config) (bool, error) {
		return certstore.New(cfg).IsInstalled(ctx)
	}
)

type Status struct {
	Running          bool      `json:"running"`
	Mode             string    `json:"mode"`
	ListenAddr       string    `json:"listen_addr"`
	PACURL           string    `json:"pac_url,omitempty"`
	HostsHTTP        string    `json:"hosts_http,omitempty"`
	HostsHTTPS       string    `json:"hosts_https,omitempty"`
	ResolverMode     string    `json:"resolver_mode,omitempty"`
	ResolverServers  []string  `json:"resolver_servers,omitempty"`
	UpstreamProfiles int       `json:"upstream_profiles,omitempty"`
	Rollback         bool      `json:"rollback"`
	CertInstalled    bool      `json:"cert_installed,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	RuleCount        int       `json:"rule_count"`
	ActiveConns      int64     `json:"active_conns"`
}

type Engine struct {
	mu     sync.Mutex
	cfg    config.Config
	logger *slog.Logger

	proxy            *proxy.Server
	startedAt        time.Time
	lastErr          error
	ruleCount        int
	running          bool
	pac              *pac.Server
	pacURL           string
	reverse          *reverse.Server
	certOK           bool
	resolver         resolver.Config
	upstreamProfiles int
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

	matcher, err := e.buildMatcher()
	if err != nil {
		return fmt.Errorf("build rules matcher: %w", err)
	}

	resolverCfg := effectiveResolverConfig(e.cfg)
	if e.cfg.Mode == config.ModeHosts && usesDirectUpstream(e.cfg.Upstream.Type) && resolverCfg.Mode == config.ResolverDoH {
		e.logger.Info("hosts mode uses DoH resolver to avoid local hosts loopback", "servers", len(resolverCfg.Servers))
	}
	dnsResolver, err := resolver.New(resolverCfg)
	if err != nil {
		return fmt.Errorf("build resolver: %w", err)
	}
	upstreamCfg := upstream.ConfigFromApp(e.cfg)
	dialer, err := upstream.NewDialer(upstreamCfg, dnsResolver)
	if err != nil {
		return fmt.Errorf("build upstream dialer: %w", err)
	}
	var proxyServer *proxy.Server
	var pacServer *pac.Server
	var pacURL string
	var reverseServer *reverse.Server
	var certOK bool

	if e.cfg.Mode == config.ModeHosts {
		certManager := certstore.New(certstore.ConfigFromApp(e.cfg))
		certOK, err = isCertInstalled(ctx, certstore.ConfigFromApp(e.cfg))
		if err != nil {
			return fmt.Errorf("check root CA install: %w", err)
		}
		if !certOK {
			return fmt.Errorf("local root CA is not installed; run `steam-accelerator cert install` first")
		}
		entries, skipped, err := hosts.EntriesFromRules(matcher.Rules(), e.cfg.Hosts.MapIP)
		if err != nil {
			return fmt.Errorf("build hosts entries: %w", err)
		}
		if len(skipped) > 0 {
			e.logger.Info("hosts mode skipped wildcard rules", "count", len(skipped))
		}
		hostsCfg := hosts.ConfigFromApp(e.cfg, entries)
		if err := preflightHosts(ctx, hostsCfg); err != nil {
			return fmt.Errorf("preflight hosts mode: %w", err)
		}
		reverseServer = reverse.New(reverse.ConfigFromApp(e.cfg), matcher, dialer, certManager, e.logger)
		if err := reverseServer.Start(); err != nil {
			return fmt.Errorf("start hosts reverse proxy: %w", err)
		}
		if err := applyHosts(ctx, hostsCfg); err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("apply hosts: %w", err)
		}
	} else {
		proxyServer = proxy.New(proxy.ConfigFromApp(e.cfg), matcher, dialer, e.logger)
		if err := proxyServer.Start(); err != nil {
			return err
		}
	}

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
	e.reverse = reverseServer
	e.certOK = certOK
	e.resolver = cloneResolverConfig(resolverCfg)
	e.upstreamProfiles = len(upstreamCfg.Profiles)
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
	reverseServer := e.reverse
	e.proxy = nil
	e.pac = nil
	e.pacURL = ""
	e.reverse = nil
	e.certOK = false
	e.resolver = resolver.Config{}
	e.upstreamProfiles = 0
	e.running = false
	e.mu.Unlock()

	if proxyServer == nil && reverseServer == nil {
		return nil
	}
	var err error
	if e.cfg.Mode == config.ModePAC || e.cfg.Mode == config.ModeSystem {
		restoreErr := restoreSystemProxy(ctx, e.cfg.Runtime.RollbackPath)
		if restoreErr != nil && !errors.Is(restoreErr, systemproxy.ErrNoState) {
			err = restoreErr
		}
	}
	if e.cfg.Mode == config.ModeHosts {
		restoreErr := restoreHosts(ctx, e.cfg.Runtime.RollbackPath)
		if restoreErr != nil && !errors.Is(restoreErr, hosts.ErrNoState) {
			err = restoreErr
		}
	}
	if pacServer != nil {
		if stopErr := pacServer.Stop(ctx); err == nil {
			err = stopErr
		}
	}
	if reverseServer != nil {
		if stopErr := reverseServer.Stop(ctx); err == nil {
			err = stopErr
		}
	}
	if proxyServer != nil {
		if stopErr := proxyServer.Stop(ctx); err == nil {
			err = stopErr
		}
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
		Running:          e.running,
		Mode:             e.cfg.Mode,
		PACURL:           e.pacURL,
		ResolverMode:     e.resolver.Mode,
		UpstreamProfiles: e.upstreamProfiles,
		Rollback:         hasRollbackState(e.cfg.Runtime.RollbackPath),
		CertInstalled:    e.certOK,
		StartedAt:        e.startedAt,
		RuleCount:        e.ruleCount,
	}
	status.ResolverServers = append([]string(nil), e.resolver.Servers...)
	if e.proxy != nil {
		status.ListenAddr = e.proxy.Addr()
		status.ActiveConns = e.proxy.ActiveConns()
	}
	if e.reverse != nil {
		status.HostsHTTP = e.reverse.HTTPAddr()
		status.HostsHTTPS = e.reverse.HTTPSAddr()
		status.ActiveConns = e.reverse.ActiveConns()
	}
	if e.lastErr != nil {
		status.LastError = e.lastErr.Error()
	}
	return status
}

func effectiveResolverConfig(cfg config.Config) resolver.Config {
	resolverCfg := resolver.ConfigFromApp(cfg)
	if cfg.Mode != config.ModeHosts || !usesDirectUpstream(cfg.Upstream.Type) {
		return resolverCfg
	}
	if resolverCfg.Mode == config.ResolverSystem {
		resolverCfg.Mode = config.ResolverDoH
		resolverCfg.Servers = config.DefaultDoHServers()
		return resolverCfg
	}
	if resolverCfg.Mode == config.ResolverDoH && len(resolverCfg.Servers) == 0 {
		resolverCfg.Servers = config.DefaultDoHServers()
	}
	return resolverCfg
}

func usesDirectUpstream(upstreamType string) bool {
	return upstreamType == "" || upstreamType == config.UpstreamDirect
}

func cloneResolverConfig(cfg resolver.Config) resolver.Config {
	cfg.Servers = append([]string(nil), cfg.Servers...)
	return cfg
}

func (e *Engine) buildMatcher() (*rules.Matcher, error) {
	groups := []rules.RuleGroup(nil)
	if e.cfg.Rules.EnableDefaultSteamRules {
		groups = rules.DefaultSteamRules
	}
	custom := append([]string(nil), e.cfg.Rules.CustomDomains...)
	if e.cfg.Mode == config.ModeHosts {
		custom = append(custom, e.cfg.Hosts.ExtraDomains...)
	}
	return rules.NewMatcher(groups, custom)
}

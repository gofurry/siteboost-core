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
	ensureCertTrusted = func(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
		return certstore.New(cfg).EnsureTrusted(ctx)
	}
	runStartupProbes = func(ctx context.Context, dialer *upstream.DirectDialer) []upstream.ProbeResult {
		return dialer.ProbeHTTPS(ctx, upstream.DefaultSteamProbeTargets(), upstream.ProbeOptions{})
	}
)

type SystemChange struct {
	Component string `json:"component"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

type Status struct {
	Running          bool                   `json:"running"`
	Mode             string                 `json:"mode"`
	ListenAddr       string                 `json:"listen_addr"`
	PACURL           string                 `json:"pac_url,omitempty"`
	HostsHTTP        string                 `json:"hosts_http,omitempty"`
	HostsHTTPS       string                 `json:"hosts_https,omitempty"`
	ResolverMode     string                 `json:"resolver_mode,omitempty"`
	ResolverServers  []string               `json:"resolver_servers,omitempty"`
	UpstreamProfiles int                    `json:"upstream_profiles,omitempty"`
	Rollback         bool                   `json:"rollback"`
	CertInstalled    bool                   `json:"cert_installed,omitempty"`
	StartupProbes    []upstream.ProbeResult `json:"startup_probes,omitempty"`
	StartedAt        time.Time              `json:"started_at,omitempty"`
	LastError        string                 `json:"last_error,omitempty"`
	RuleSetName      string                 `json:"rule_set_name,omitempty"`
	RuleSetVersion   string                 `json:"rule_set_version,omitempty"`
	RuleSetUpdatedAt string                 `json:"rule_set_updated_at,omitempty"`
	RuleCount        int                    `json:"rule_count"`
	ActiveConns      int64                  `json:"active_conns"`
	SystemChanges    []SystemChange         `json:"system_changes,omitempty"`
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
	startupProbes    []upstream.ProbeResult
	ruleSet          rules.RuleSetInfo
	systemChanges    []SystemChange
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
	var startupProbes []upstream.ProbeResult
	var systemChanges []SystemChange

	if e.cfg.Mode == config.ModeHosts {
		certCfg := certstore.ConfigFromApp(e.cfg)
		certManager := certstore.New(certCfg)
		certOK, err = isCertInstalled(ctx, certCfg)
		if err != nil {
			return fmt.Errorf("check root CA install: %w", err)
		}
		if !certOK {
			if !e.cfg.Cert.AutoInstall {
				return fmt.Errorf("local root CA is not installed; run `steam-accelerator cert install` first or enable cert.auto_install")
			}
			trust, err := ensureCertTrusted(ctx, certCfg)
			if err != nil {
				return fmt.Errorf("install local root CA: %w", err)
			}
			certOK = true
			systemChanges = append(systemChanges, certTrustChange(trust))
		} else {
			systemChanges = append(systemChanges, SystemChange{Component: "root_ca", Action: "check", Status: "already_trusted", Detail: fmt.Sprintf("store=%s", certCfg.StoreScope)})
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
		systemChanges = append(systemChanges, SystemChange{Component: "hosts", Action: "preflight", Status: "ok"})
		if directDialer, ok := dialer.(*upstream.DirectDialer); ok && usesDirectUpstream(e.cfg.Upstream.Type) && len(upstreamCfg.Profiles) > 0 {
			startupProbes = runStartupProbes(ctx, directDialer)
			logStartupProbes(e.logger, startupProbes)
		}
		reverseServer = reverse.New(reverse.ConfigFromApp(e.cfg), matcher, dialer, certManager, e.logger)
		if err := reverseServer.Start(); err != nil {
			return fmt.Errorf("start hosts reverse proxy: %w", err)
		}
		systemChanges = append(systemChanges, SystemChange{Component: "reverse_proxy", Action: "listen", Status: "ok"})
		if err := applyHosts(ctx, hostsCfg); err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("apply hosts: %w", err)
		}
		systemChanges = append(systemChanges, SystemChange{Component: "hosts", Action: "apply", Status: "ok", Detail: fmt.Sprintf("entries=%d", len(entries))})
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
	e.startupProbes = cloneStartupProbes(startupProbes)
	e.ruleSet = ruleSetInfo(e.cfg, matcher)
	e.systemChanges = cloneSystemChanges(systemChanges)
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
	e.startupProbes = nil
	e.ruleSet = rules.RuleSetInfo{}
	e.systemChanges = nil
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
		RuleSetName:      e.ruleSet.Name,
		RuleSetVersion:   e.ruleSet.Version,
		RuleSetUpdatedAt: e.ruleSet.UpdatedAt,
	}
	status.ResolverServers = append([]string(nil), e.resolver.Servers...)
	status.StartupProbes = cloneStartupProbes(e.startupProbes)
	status.SystemChanges = cloneSystemChanges(e.systemChanges)
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

func cloneStartupProbes(probes []upstream.ProbeResult) []upstream.ProbeResult {
	if len(probes) == 0 {
		return nil
	}
	return append([]upstream.ProbeResult(nil), probes...)
}

func cloneSystemChanges(changes []SystemChange) []SystemChange {
	if len(changes) == 0 {
		return nil
	}
	return append([]SystemChange(nil), changes...)
}

func certTrustChange(trust certstore.TrustResult) SystemChange {
	change := SystemChange{Component: "root_ca", Action: "install", Status: "ok", Detail: fmt.Sprintf("store=%s", trust.StoreScope)}
	if trust.AlreadyTrusted {
		change.Action = "check"
		change.Status = "already_trusted"
	}
	if trust.Changed {
		change.Detail += ",installed"
	}
	return change
}

func ruleSetInfo(cfg config.Config, matcher *rules.Matcher) rules.RuleSetInfo {
	if cfg.Rules.EnableDefaultSteamRules {
		return rules.DefaultSteamRuleSetInfo()
	}
	compiled := matcher.Rules()
	return rules.RuleSetInfo{
		Name:          "custom",
		GroupCount:    1,
		ExactCount:    len(compiled.Exact),
		WildcardCount: len(compiled.Wildcard),
	}
}

func logStartupProbes(logger *slog.Logger, probes []upstream.ProbeResult) {
	if len(probes) == 0 {
		return
	}
	okCount := 0
	for _, probe := range probes {
		if probe.OK {
			okCount++
			continue
		}
		logger.Warn("startup steam probe failed",
			"host", probe.Host,
			"target", probe.Target,
			"stage", probe.Stage,
			"error", probe.Error,
			"duration_ms", probe.DurationMillis,
		)
	}
	logger.Info("startup steam probes completed", "ok", okCount, "failed", len(probes)-okCount)
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

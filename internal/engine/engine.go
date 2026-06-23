package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/dnsintercept"
	"github.com/gofurry/go-steam-core/internal/hosts"
	"github.com/gofurry/go-steam-core/internal/pac"
	"github.com/gofurry/go-steam-core/internal/privilege"
	"github.com/gofurry/go-steam-core/internal/provider"
	"github.com/gofurry/go-steam-core/internal/proxy"
	"github.com/gofurry/go-steam-core/internal/resolver"
	"github.com/gofurry/go-steam-core/internal/reverse"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/systemdns"
	"github.com/gofurry/go-steam-core/internal/systemproxy"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

var (
	applySystemProxy   = systemproxy.Apply
	restoreSystemProxy = systemproxy.Restore
	hasRollbackState   = systemproxy.HasState
	preflightHosts     = hosts.Preflight
	applyHosts         = hosts.Apply
	restoreHosts       = privilege.RestoreHosts
	preflightSystemDNS = privilege.PreflightSystemDNS
	applySystemDNS     = privilege.ApplySystemDNS
	restoreSystemDNS   = privilege.RestoreSystemDNS
	newDNSIntercept    = func(cfg dnsintercept.Config, matcher *rules.Matcher, forwarder dnsintercept.Forwarder) (dnsInterceptServer, error) {
		return dnsintercept.New(cfg, matcher, forwarder)
	}
	isCertInstalled = func(ctx context.Context, cfg certstore.Config) (bool, error) {
		return certstore.New(cfg).IsInstalled(ctx)
	}
	ensureCertTrusted    = privilege.EnsureCertTrusted
	prepareHostsStart    = privilege.PrepareHostsStart
	prepareHostsElevated = privilege.PrepareHostsStartElevated
	shouldUseHostHelper  = privilege.ShouldUseHelper
	runStartupProbes     = func(ctx context.Context, dialer *upstream.DirectDialer, targets []upstream.ProbeTarget) []upstream.ProbeResult {
		return dialer.ProbeHTTPS(ctx, targets, upstream.ProbeOptions{})
	}
)

type dnsInterceptServer interface {
	Start() error
	Stop(ctx context.Context) error
	Status() dnsintercept.Status
}

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
	DNSIntercept     *dnsintercept.Status   `json:"dns_intercept,omitempty"`
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
	Providers        []provider.Summary     `json:"providers,omitempty"`
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
	dnsIntercept     dnsInterceptServer
	certOK           bool
	resolver         resolver.Config
	upstreamProfiles int
	startupProbes    []upstream.ProbeResult
	ruleSet          rules.RuleSetInfo
	providers        []provider.Summary
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
	enabledProviders, err := e.enabledProviders()
	if err != nil {
		return err
	}

	resolverCfg := effectiveResolverConfig(e.cfg)
	if requiresLoopSafeResolver(e.cfg) && resolverCfg.Mode == config.ResolverDoH {
		e.logger.Info("mode uses DoH resolver to avoid local DNS loopback", "mode", e.cfg.Mode, "servers", len(resolverCfg.Servers))
	}
	dnsResolver, err := resolver.New(resolverCfg)
	if err != nil {
		return fmt.Errorf("build resolver: %w", err)
	}
	providerProfiles := []upstream.Profile(nil)
	if (e.cfg.Mode == config.ModeHosts || e.cfg.Mode == config.ModeDNS) && usesDirectUpstream(e.cfg.Upstream.Type) {
		providerProfiles = provider.OutboundProfiles(enabledProviders)
	}
	upstreamCfg := upstream.ConfigFromApp(e.cfg, providerProfiles)
	dialer, err := upstream.NewDialer(upstreamCfg, dnsResolver)
	if err != nil {
		return fmt.Errorf("build upstream dialer: %w", err)
	}
	var proxyServer *proxy.Server
	var pacServer *pac.Server
	var pacURL string
	var reverseServer *reverse.Server
	var dnsServer dnsInterceptServer
	var certOK bool
	var startupProbes []upstream.ProbeResult
	var systemChanges []SystemChange

	if e.cfg.Mode == config.ModeHosts {
		certCfg := certstore.ConfigFromApp(e.cfg)
		certManager := certstore.New(certCfg)
		entries, skipped, err := hosts.EntriesFromRules(matcher.Rules(), e.cfg.Hosts.MapIP)
		if err != nil {
			return fmt.Errorf("build hosts entries: %w", err)
		}
		if len(skipped) > 0 {
			e.logger.Info("hosts mode skipped wildcard rules", "count", len(skipped))
		}
		hostsCfg := hosts.ConfigFromApp(e.cfg, entries)
		probeTargets := provider.ProbeTargets(enabledProviders)
		if directDialer, ok := dialer.(*upstream.DirectDialer); ok && usesDirectUpstream(e.cfg.Upstream.Type) && len(probeTargets) > 0 {
			startupProbes = runStartupProbes(ctx, directDialer, probeTargets)
			logStartupProbes(e.logger, startupProbes)
		}
		reverseServer = reverse.New(reverse.ConfigFromApp(e.cfg), matcher, dialer, certManager, e.logger)
		if err := reverseServer.Start(); err != nil {
			return fmt.Errorf("start hosts reverse proxy: %w", err)
		}
		systemChanges = append(systemChanges, SystemChange{Component: "reverse_proxy", Action: "listen", Status: "ok"})
		prepare, err := e.prepareHostsForStart(ctx, certManager, certCfg, hostsCfg, shouldUseHostHelper())
		if err != nil {
			_ = reverseServer.Stop(context.Background())
			return err
		}
		certOK = prepare.CertTrusted
		systemChanges = append(systemChanges, certTrustChange(prepare.Cert))
		preflightDetail := ""
		applyDetail := fmt.Sprintf("entries=%d", prepare.Entries)
		if prepare.Cert.ViaHelper {
			preflightDetail = "helper=elevated"
			applyDetail += ",helper=elevated"
		}
		systemChanges = append(systemChanges, SystemChange{Component: "hosts", Action: "preflight", Status: "ok", Detail: preflightDetail})
		systemChanges = append(systemChanges, SystemChange{Component: "hosts", Action: "apply", Status: "ok", Detail: applyDetail})
	} else if e.cfg.Mode == config.ModeDNS {
		certCfg := certstore.ConfigFromApp(e.cfg)
		certManager := certstore.New(certCfg)
		var systemDNSCfg systemdns.Config
		if usesSystemDNS(e.cfg) {
			systemDNSCfg, err = systemdns.ConfigFromApp(e.cfg)
			if err != nil {
				return fmt.Errorf("build system DNS config: %w", err)
			}
			preflight, err := preflightSystemDNS(ctx, systemDNSCfg)
			if err != nil {
				return fmt.Errorf("preflight system DNS: %w", err)
			}
			systemChanges = append(systemChanges, systemDNSChange("preflight", preflight))
		}
		probeTargets := provider.ProbeTargets(enabledProviders)
		if directDialer, ok := dialer.(*upstream.DirectDialer); ok && usesDirectUpstream(e.cfg.Upstream.Type) && len(probeTargets) > 0 {
			startupProbes = runStartupProbes(ctx, directDialer, probeTargets)
			logStartupProbes(e.logger, startupProbes)
		}
		reverseServer = reverse.New(reverse.ConfigFromApp(e.cfg), matcher, dialer, certManager, e.logger)
		if err := reverseServer.Start(); err != nil {
			return fmt.Errorf("start DNS reverse proxy: %w", err)
		}
		dnsCfg, err := dnsintercept.ConfigFromApp(e.cfg)
		if err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("build DNS intercept config: %w", err)
		}
		forwarder, err := dnsintercept.NewRawForwarder(resolverCfg)
		if err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("build DNS intercept forwarder: %w", err)
		}
		dnsServer, err = newDNSIntercept(dnsCfg, matcher, forwarder)
		if err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("build DNS intercept server: %w", err)
		}
		if err := dnsServer.Start(); err != nil {
			_ = reverseServer.Stop(context.Background())
			return fmt.Errorf("start DNS intercept: %w", err)
		}
		if usesSystemDNS(e.cfg) {
			apply, err := applySystemDNS(ctx, systemDNSCfg)
			if err != nil {
				restoreErr := restoreSystemDNS(context.Background(), e.cfg.Runtime.RollbackPath)
				_ = dnsServer.Stop(context.Background())
				_ = reverseServer.Stop(context.Background())
				if restoreErr != nil && !errors.Is(restoreErr, systemdns.ErrNoState) {
					return fmt.Errorf("%w; system DNS restore after failed apply also failed: %v", err, restoreErr)
				}
				return err
			}
			systemChanges = append(systemChanges, systemDNSChange("apply", apply))
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
	e.dnsIntercept = dnsServer
	e.certOK = certOK
	e.resolver = cloneResolverConfig(resolverCfg)
	e.upstreamProfiles = len(upstreamCfg.Profiles)
	e.startupProbes = cloneStartupProbes(startupProbes)
	e.ruleSet = ruleSetInfo(enabledProviders, matcher)
	e.providers = provider.Summaries(enabledProviders)
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
	dnsServer := e.dnsIntercept
	e.proxy = nil
	e.pac = nil
	e.pacURL = ""
	e.reverse = nil
	e.dnsIntercept = nil
	e.certOK = false
	e.resolver = resolver.Config{}
	e.upstreamProfiles = 0
	e.startupProbes = nil
	e.ruleSet = rules.RuleSetInfo{}
	e.providers = nil
	e.systemChanges = nil
	e.running = false
	e.mu.Unlock()

	if proxyServer == nil && reverseServer == nil && dnsServer == nil {
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
	if e.cfg.Mode == config.ModeDNS && e.cfg.DNS.Strategy == config.DNSInterceptSystem {
		restoreErr := restoreSystemDNS(ctx, e.cfg.Runtime.RollbackPath)
		if restoreErr != nil && !errors.Is(restoreErr, systemdns.ErrNoState) {
			err = restoreErr
		}
	}
	if pacServer != nil {
		if stopErr := pacServer.Stop(ctx); err == nil {
			err = stopErr
		}
	}
	if dnsServer != nil {
		if stopErr := dnsServer.Stop(ctx); err == nil {
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
		Providers:        cloneProviderSummaries(e.providers),
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
	if e.dnsIntercept != nil {
		dnsStatus := e.dnsIntercept.Status()
		if e.cfg.DNS.Strategy == config.DNSInterceptSystem {
			dnsStatus.SystemDNS = e.running
		}
		status.DNSIntercept = &dnsStatus
	}
	if e.lastErr != nil {
		status.LastError = e.lastErr.Error()
	}
	return status
}

func effectiveResolverConfig(cfg config.Config) resolver.Config {
	resolverCfg := resolver.ConfigFromApp(cfg)
	if !requiresLoopSafeResolver(cfg) {
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

func requiresLoopSafeResolver(cfg config.Config) bool {
	return cfg.Mode == config.ModeDNS || (cfg.Mode == config.ModeHosts && usesDirectUpstream(cfg.Upstream.Type))
}

func usesDirectUpstream(upstreamType string) bool {
	return upstreamType == "" || upstreamType == config.UpstreamDirect
}

func usesSystemDNS(cfg config.Config) bool {
	return cfg.Mode == config.ModeDNS && cfg.DNS.Strategy == config.DNSInterceptSystem
}

func (e *Engine) prepareHostsForStart(ctx context.Context, certManager *certstore.Manager, certCfg certstore.Config, hostsCfg hosts.Config, preferHelper bool) (privilege.PrepareHostsResult, error) {
	if preferHelper {
		return prepareHostsWithHelper(ctx, certManager, certCfg, hostsCfg, e.cfg.Cert.AutoInstall)
	}
	prepare, err := prepareHostsDirect(ctx, certCfg, hostsCfg, e.cfg.Cert.AutoInstall)
	if err == nil {
		return prepare, nil
	}
	if !shouldRetryHostsWithHelper(err) {
		return privilege.PrepareHostsResult{}, err
	}
	e.logger.Info("direct hosts system change failed; retrying with elevated helper", "error", err)
	prepare, helperErr := prepareHostsWithHelper(ctx, certManager, certCfg, hostsCfg, e.cfg.Cert.AutoInstall)
	if helperErr != nil {
		return privilege.PrepareHostsResult{}, fmt.Errorf("%w; elevated helper retry failed: %v", err, helperErr)
	}
	return prepare, nil
}

func prepareHostsWithHelper(ctx context.Context, certManager *certstore.Manager, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (privilege.PrepareHostsResult, error) {
	if _, err := certManager.EnsureRootCA(); err != nil {
		return privilege.PrepareHostsResult{}, fmt.Errorf("ensure local root CA before elevated helper: %w", err)
	}
	return prepareHostsElevated(ctx, certCfg, hostsCfg, autoInstall)
}

func prepareHostsDirect(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (privilege.PrepareHostsResult, error) {
	certOK, err := isCertInstalled(ctx, certCfg)
	if err != nil {
		return privilege.PrepareHostsResult{}, fmt.Errorf("check root CA install: %w", err)
	}
	var trust certstore.TrustResult
	if !certOK {
		if !autoInstall {
			return privilege.PrepareHostsResult{}, fmt.Errorf("local root CA is not installed; run `steam-accelerator cert install` first or enable cert.auto_install")
		}
		trust, err = ensureCertTrusted(ctx, certCfg)
		if err != nil {
			return privilege.PrepareHostsResult{}, fmt.Errorf("install local root CA: %w", err)
		}
	} else {
		trust = certstore.TrustResult{StoreScope: certCfg.StoreScope, AlreadyTrusted: true}
	}
	if err := preflightHosts(ctx, hostsCfg); err != nil {
		return privilege.PrepareHostsResult{}, fmt.Errorf("preflight hosts mode: %w", err)
	}
	if err := applyHosts(ctx, hostsCfg); err != nil {
		return privilege.PrepareHostsResult{}, fmt.Errorf("apply hosts: %w", err)
	}
	return privilege.PrepareHostsResult{Cert: trust, CertTrusted: true, Entries: len(hostsCfg.Entries)}, nil
}

func shouldRetryHostsWithHelper(err error) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "administrator")
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

func cloneProviderSummaries(providers []provider.Summary) []provider.Summary {
	if len(providers) == 0 {
		return nil
	}
	return append([]provider.Summary(nil), providers...)
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
	if trust.ViaHelper {
		change.Detail += ",helper=elevated"
	}
	return change
}

func systemDNSChange(action string, result systemdns.Result) SystemChange {
	detail := fmt.Sprintf("interfaces=%d", result.Interfaces)
	if len(result.ServerIPs) > 0 {
		detail += ",dns=" + strings.Join(result.ServerIPs, "|")
	}
	if result.ViaHelper {
		detail += ",helper=elevated"
	}
	return SystemChange{Component: "system_dns", Action: action, Status: "ok", Detail: detail}
}

func ruleSetInfo(providers []provider.Provider, matcher *rules.Matcher) rules.RuleSetInfo {
	if len(providers) == 1 {
		return provider.RuleSetInfo(providers[0])
	}
	if len(providers) > 1 {
		compiled := matcher.Rules()
		return rules.RuleSetInfo{
			Name:          "multi-provider",
			GroupCount:    len(provider.RuleGroups(providers)),
			ExactCount:    len(compiled.Exact),
			WildcardCount: len(compiled.Wildcard),
		}
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
		logger.Warn("startup provider probe failed",
			"provider", probe.ProviderID,
			"host", probe.Host,
			"target", probe.Target,
			"stage", probe.Stage,
			"error", probe.Error,
			"duration_ms", probe.DurationMillis,
		)
	}
	logger.Info("startup provider probes completed", "ok", okCount, "failed", len(probes)-okCount)
}

func (e *Engine) buildMatcher() (*rules.Matcher, error) {
	enabledProviders, err := e.enabledProviders()
	if err != nil {
		return nil, err
	}
	groups := provider.RuleGroups(enabledProviders)
	custom := append([]string(nil), e.cfg.Rules.CustomDomains...)
	if e.cfg.Mode == config.ModeHosts || e.cfg.Mode == config.ModeDNS {
		custom = append(custom, e.cfg.Hosts.ExtraDomains...)
	}
	return rules.NewMatcher(groups, custom)
}

func (e *Engine) enabledProviders() ([]provider.Provider, error) {
	providers, err := provider.ResolveEnabled(e.cfg.Providers.Enabled)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

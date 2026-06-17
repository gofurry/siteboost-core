package resolver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/miekg/dns"
)

type Resolver interface {
	Resolve(ctx context.Context, host string) ([]net.IP, error)
}

type Config struct {
	Mode        string
	Servers     []string
	PreferIPv4  bool
	PreferIPv6  bool
	DisableIPv6 bool
	CacheTTL    time.Duration
	Timeout     time.Duration
}

type Client struct {
	cfg        Config
	cache      map[string]cacheEntry
	mu         sync.Mutex
	httpClient *http.Client
}

type cacheEntry struct {
	ips    []net.IP
	expire time.Time
}

func ConfigFromApp(cfg config.Config) Config {
	return Config{
		Mode:        cfg.Resolver.Mode,
		Servers:     append([]string(nil), cfg.Resolver.Servers...),
		PreferIPv4:  cfg.Resolver.PreferIPv4,
		PreferIPv6:  cfg.Resolver.PreferIPv6,
		DisableIPv6: cfg.Resolver.DisableIPv6,
		CacheTTL:    cfg.Resolver.CacheTTL.Std(),
		Timeout:     cfg.Resolver.Timeout.Std(),
	}
}

func New(cfg Config) (*Client, error) {
	if cfg.Mode == "" {
		cfg.Mode = config.ResolverSystem
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 10 * time.Minute
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{
		cfg:   cfg,
		cache: make(map[string]cacheEntry),
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (c *Client) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	normalized, err := rules.NormalizeHost(host)
	if err != nil {
		return nil, fmt.Errorf("normalize host: %w", err)
	}
	if ip := net.ParseIP(normalized); ip != nil {
		ips := c.applyPolicy([]net.IP{ip})
		if len(ips) == 0 {
			return nil, fmt.Errorf("no usable IPs for %s", normalized)
		}
		return ips, nil
	}

	if ips, ok := c.getCache(normalized); ok {
		return ips, nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	ips, ttl, err := c.resolveFresh(ctx, normalized)
	if err != nil {
		return nil, err
	}
	ips = c.applyPolicy(ips)
	if len(ips) == 0 {
		return nil, fmt.Errorf("no usable IPs for %s", normalized)
	}
	if ttl <= 0 {
		ttl = c.cfg.CacheTTL
	}
	if ttl > c.cfg.CacheTTL {
		ttl = c.cfg.CacheTTL
	}
	c.putCache(normalized, ips, ttl)
	return cloneIPs(ips), nil
}

func (c *Client) resolveFresh(ctx context.Context, host string) ([]net.IP, time.Duration, error) {
	switch c.cfg.Mode {
	case config.ResolverSystem:
		return c.resolveSystem(ctx, host)
	case config.ResolverUDP, config.ResolverTCP:
		return c.resolveDNS(ctx, host, c.cfg.Mode)
	case config.ResolverDoH:
		return c.resolveDoH(ctx, host)
	default:
		return nil, 0, fmt.Errorf("unsupported resolver mode %q", c.cfg.Mode)
	}
}

func (c *Client) resolveSystem(ctx context.Context, host string) ([]net.IP, time.Duration, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, 0, fmt.Errorf("system resolve %s: %w", host, err)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}
	return ips, c.cfg.CacheTTL, nil
}

func (c *Client) resolveDNS(ctx context.Context, host, network string) ([]net.IP, time.Duration, error) {
	return c.resolveByQueries(ctx, host, func(ctx context.Context, qtype uint16) ([]net.IP, time.Duration, error) {
		var lastErr error
		for _, server := range c.cfg.Servers {
			msg := newQuestion(host, qtype)
			client := &dns.Client{Net: network, Timeout: c.cfg.Timeout}
			resp, _, err := client.ExchangeContext(ctx, msg, normalizeDNSServer(server))
			if err != nil {
				lastErr = err
				continue
			}
			ips, ttl, err := parseDNSResponse(resp, qtype)
			if err != nil {
				lastErr = err
				continue
			}
			return ips, ttl, nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no DNS servers configured")
		}
		return nil, 0, lastErr
	})
}

func (c *Client) resolveDoH(ctx context.Context, host string) ([]net.IP, time.Duration, error) {
	return c.resolveByQueries(ctx, host, func(ctx context.Context, qtype uint16) ([]net.IP, time.Duration, error) {
		var lastErr error
		for _, server := range c.cfg.Servers {
			ips, ttl, err := c.queryDoH(ctx, server, host, qtype)
			if err != nil {
				lastErr = err
				continue
			}
			return ips, ttl, nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no DoH servers configured")
		}
		return nil, 0, lastErr
	})
}

func (c *Client) resolveByQueries(ctx context.Context, host string, query func(context.Context, uint16) ([]net.IP, time.Duration, error)) ([]net.IP, time.Duration, error) {
	var all []net.IP
	ttl := c.cfg.CacheTTL
	var errs []string

	a, aTTL, err := query(ctx, dns.TypeA)
	if err != nil {
		errs = append(errs, err.Error())
	} else {
		all = append(all, a...)
		ttl = minDuration(ttl, aTTL)
	}

	if !c.cfg.DisableIPv6 {
		aaaa, aaaaTTL, err := query(ctx, dns.TypeAAAA)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			all = append(all, aaaa...)
			ttl = minDuration(ttl, aaaaTTL)
		}
	}

	if len(all) == 0 {
		return nil, 0, fmt.Errorf("resolve %s failed: %s", host, strings.Join(errs, "; "))
	}
	return all, ttl, nil
}

func (c *Client) queryDoH(ctx context.Context, server, host string, qtype uint16) ([]net.IP, time.Duration, error) {
	endpoint, err := url.Parse(server)
	if err != nil {
		return nil, 0, err
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, 0, fmt.Errorf("DoH server %q must use http or https", server)
	}

	wire, err := newQuestion(host, qtype).Pack()
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(wire))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("DoH server returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, 0, err
	}
	return parseDNSResponse(msg, qtype)
}

func (c *Client) getCache(host string) ([]net.IP, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[host]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expire) {
		delete(c.cache, host)
		return nil, false
	}
	return cloneIPs(entry.ips), true
}

func (c *Client) putCache(host string, ips []net.IP, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[host] = cacheEntry{
		ips:    cloneIPs(ips),
		expire: time.Now().Add(ttl),
	}
}

func (c *Client) applyPolicy(ips []net.IP) []net.IP {
	filtered := make([]net.IP, 0, len(ips))
	seen := make(map[string]struct{})
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		if c.cfg.DisableIPv6 && ip.To4() == nil {
			continue
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, append(net.IP(nil), ip...))
	}

	if c.cfg.PreferIPv4 {
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].To4() != nil && filtered[j].To4() == nil
		})
	}
	if c.cfg.PreferIPv6 {
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].To4() == nil && filtered[j].To4() != nil
		})
	}
	return filtered
}

func newQuestion(host string, qtype uint16) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), qtype)
	msg.RecursionDesired = true
	return msg
}

func parseDNSResponse(resp *dns.Msg, qtype uint16) ([]net.IP, time.Duration, error) {
	if resp == nil {
		return nil, 0, fmt.Errorf("empty DNS response")
	}
	if resp.Rcode != dns.RcodeSuccess {
		return nil, 0, fmt.Errorf("DNS response code %s", dns.RcodeToString[resp.Rcode])
	}

	var ips []net.IP
	var ttl time.Duration
	for _, rr := range resp.Answer {
		header := rr.Header()
		if header == nil {
			continue
		}
		switch record := rr.(type) {
		case *dns.A:
			if qtype == dns.TypeA {
				ips = append(ips, record.A)
				ttl = minPositiveTTL(ttl, header.Ttl)
			}
		case *dns.AAAA:
			if qtype == dns.TypeAAAA {
				ips = append(ips, record.AAAA)
				ttl = minPositiveTTL(ttl, header.Ttl)
			}
		}
	}
	if len(ips) == 0 {
		return nil, 0, fmt.Errorf("DNS response has no %s records", dns.TypeToString[qtype])
	}
	return ips, ttl, nil
}

func normalizeDNSServer(server string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(strings.Trim(server, "[]"), "53")
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func minPositiveTTL(current time.Duration, ttl uint32) time.Duration {
	next := time.Duration(ttl) * time.Second
	if current == 0 || next < current {
		return next
	}
	return current
}

func cloneIPs(ips []net.IP) []net.IP {
	cloned := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		cloned = append(cloned, append(net.IP(nil), ip...))
	}
	return cloned
}

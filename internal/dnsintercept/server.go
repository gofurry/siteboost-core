package dnsintercept

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/resolver"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/miekg/dns"
)

type Config struct {
	Strategy          string
	ListenAddr        string
	MapIPv4           net.IP
	MapIPv6           net.IP
	TTL               time.Duration
	CacheTTL          time.Duration
	BlockHTTPSRecords bool
	ForwardTimeout    time.Duration
}

type Status struct {
	Strategy         string `json:"strategy"`
	ListenAddr       string `json:"listen_addr"`
	SystemDNS        bool   `json:"system_dns"`
	TargetQueries    uint64 `json:"target_queries"`
	ForwardedQueries uint64 `json:"forwarded_queries"`
	CacheHits        uint64 `json:"cache_hits"`
	BlockedQueries   uint64 `json:"blocked_queries"`
	ErrorQueries     uint64 `json:"error_queries"`
}

type Forwarder interface {
	Forward(ctx context.Context, req *dns.Msg) (*dns.Msg, error)
}

type Server struct {
	cfg       Config
	matcher   *rules.Matcher
	forwarder Forwarder

	mu         sync.RWMutex
	udpServer  *dns.Server
	tcpServer  *dns.Server
	listenAddr string

	cacheMu sync.Mutex
	cache   map[string]cacheEntry

	targetQueries    atomic.Uint64
	forwardedQueries atomic.Uint64
	cacheHits        atomic.Uint64
	blockedQueries   atomic.Uint64
	errorQueries     atomic.Uint64
}

type cacheEntry struct {
	resp    *dns.Msg
	expires time.Time
}

func ConfigFromApp(cfg config.Config) (Config, error) {
	dnsCfg := cfg.DNS
	out := Config{
		Strategy:          dnsCfg.Strategy,
		ListenAddr:        strings.TrimSpace(dnsCfg.ListenAddr),
		TTL:               dnsCfg.TTL.Std(),
		CacheTTL:          cfg.Resolver.CacheTTL.Std(),
		BlockHTTPSRecords: dnsCfg.BlockHTTPSRecords,
		ForwardTimeout:    cfg.Resolver.Timeout.Std(),
	}
	if dnsCfg.MapIPv4 != "" {
		out.MapIPv4 = net.ParseIP(dnsCfg.MapIPv4).To4()
	}
	if dnsCfg.MapIPv6 != "" {
		out.MapIPv6 = net.ParseIP(dnsCfg.MapIPv6).To16()
	}
	if out.ForwardTimeout <= 0 {
		out.ForwardTimeout = 5 * time.Second
	}
	if out.TTL <= 0 {
		out.TTL = 30 * time.Second
	}
	return out, nil
}

func New(cfg Config, matcher *rules.Matcher, forwarder Forwarder) (*Server, error) {
	if matcher == nil {
		return nil, errors.New("dns intercept matcher is required")
	}
	if forwarder == nil {
		return nil, errors.New("dns intercept forwarder is required")
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return nil, errors.New("dns intercept listen address is required")
	}
	if cfg.MapIPv4 == nil && cfg.MapIPv6 == nil {
		return nil, errors.New("dns intercept requires at least one map address")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.ForwardTimeout <= 0 {
		cfg.ForwardTimeout = 5 * time.Second
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 10 * time.Minute
	}
	return &Server{
		cfg:       cfg,
		matcher:   matcher,
		forwarder: forwarder,
		cache:     make(map[string]cacheEntry),
	}, nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udpServer != nil || s.tcpServer != nil {
		return nil
	}

	udpConn, tcpListener, listenAddr, err := listenPair(s.cfg.ListenAddr)
	if err != nil {
		return err
	}

	handler := dns.HandlerFunc(s.ServeDNS)
	s.udpServer = &dns.Server{PacketConn: udpConn, Handler: handler}
	s.tcpServer = &dns.Server{Listener: tcpListener, Handler: handler}
	s.listenAddr = listenAddr

	udpServer := s.udpServer
	tcpServer := s.tcpServer
	go func() {
		_ = udpServer.ActivateAndServe()
	}()
	go func() {
		_ = tcpServer.ActivateAndServe()
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	udpServer := s.udpServer
	tcpServer := s.tcpServer
	s.udpServer = nil
	s.tcpServer = nil
	s.listenAddr = ""
	s.mu.Unlock()

	var errs []error
	if udpServer != nil {
		if err := udpServer.ShutdownContext(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop DNS UDP server: %w", err))
		}
	}
	if tcpServer != nil {
		if err := tcpServer.ShutdownContext(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop DNS TCP server: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (s *Server) Status() Status {
	s.mu.RLock()
	listenAddr := s.listenAddr
	if listenAddr == "" {
		listenAddr = s.cfg.ListenAddr
	}
	s.mu.RUnlock()

	return Status{
		Strategy:         s.cfg.Strategy,
		ListenAddr:       listenAddr,
		SystemDNS:        false,
		TargetQueries:    s.targetQueries.Load(),
		ForwardedQueries: s.forwardedQueries.Load(),
		CacheHits:        s.cacheHits.Load(),
		BlockedQueries:   s.blockedQueries.Load(),
		ErrorQueries:     s.errorQueries.Load(),
	}
}

func (s *Server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	resp := s.handle(req)
	if err := w.WriteMsg(resp); err != nil {
		s.errorQueries.Add(1)
	}
}

func (s *Server) handle(req *dns.Msg) *dns.Msg {
	if len(req.Question) == 0 {
		resp := new(dns.Msg)
		resp.SetReply(req)
		return resp
	}

	q := req.Question[0]
	host := strings.TrimSuffix(q.Name, ".")
	if _, ok := s.matcher.MatchHost(host); ok {
		s.targetQueries.Add(1)
		if s.shouldForwardTargetQuery(q) {
			return s.forwardResponse(req)
		}
		return s.targetResponse(req, q)
	}

	return s.forwardResponse(req)
}

func (s *Server) forwardResponse(req *dns.Msg) *dns.Msg {
	if resp, ok := s.getCache(req); ok {
		s.cacheHits.Add(1)
		return resp
	}

	s.forwardedQueries.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ForwardTimeout)
	defer cancel()
	resp, err := s.forwarder.Forward(ctx, req)
	if err != nil {
		s.errorQueries.Add(1)
		return responseWithRCode(req, dns.RcodeServerFailure)
	}
	if resp == nil {
		s.errorQueries.Add(1)
		return responseWithRCode(req, dns.RcodeServerFailure)
	}
	resp.Id = req.Id
	s.putCache(req, resp)
	return resp
}

func (s *Server) shouldForwardTargetQuery(q dns.Question) bool {
	return q.Qclass == dns.ClassINET && (q.Qtype == dns.TypeHTTPS || q.Qtype == dns.TypeSVCB) && !s.cfg.BlockHTTPSRecords
}

func (s *Server) targetResponse(req *dns.Msg, q dns.Question) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true
	resp.RecursionAvailable = true

	if q.Qclass != dns.ClassINET {
		s.blockedQueries.Add(1)
		return resp
	}

	ttl := uint32(s.cfg.TTL / time.Second)
	switch q.Qtype {
	case dns.TypeA:
		if s.cfg.MapIPv4 == nil {
			s.blockedQueries.Add(1)
			return resp
		}
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
			A:   s.cfg.MapIPv4.To4(),
		})
	case dns.TypeAAAA:
		if s.cfg.MapIPv6 == nil {
			s.blockedQueries.Add(1)
			return resp
		}
		resp.Answer = append(resp.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
			AAAA: s.cfg.MapIPv6.To16(),
		})
	case dns.TypeHTTPS, dns.TypeSVCB:
		if s.cfg.BlockHTTPSRecords {
			s.blockedQueries.Add(1)
			return resp
		}
	default:
		s.blockedQueries.Add(1)
	}
	return resp
}

func responseWithRCode(req *dns.Msg, rcode int) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, rcode)
	return resp
}

func (s *Server) getCache(req *dns.Msg) (*dns.Msg, bool) {
	key, ok := cacheKey(req)
	if !ok {
		return nil, false
	}
	now := time.Now()
	s.cacheMu.Lock()
	entry, ok := s.cache[key]
	if !ok {
		s.cacheMu.Unlock()
		return nil, false
	}
	if !now.Before(entry.expires) {
		delete(s.cache, key)
		s.cacheMu.Unlock()
		return nil, false
	}
	resp := entry.resp.Copy()
	s.cacheMu.Unlock()
	resp.Id = req.Id
	capResponseTTL(resp, uint32(time.Until(entry.expires)/time.Second))
	return resp, true
}

func (s *Server) putCache(req, resp *dns.Msg) {
	if resp.Rcode != dns.RcodeSuccess {
		return
	}
	key, ok := cacheKey(req)
	if !ok {
		return
	}
	ttl := responseTTL(resp, s.cfg.CacheTTL)
	if ttl <= 0 {
		return
	}
	entry := cacheEntry{
		resp:    resp.Copy(),
		expires: time.Now().Add(ttl),
	}
	entry.resp.Id = 0
	s.cacheMu.Lock()
	s.cache[key] = entry
	s.cacheMu.Unlock()
}

func cacheKey(req *dns.Msg) (string, bool) {
	if len(req.Question) != 1 {
		return "", false
	}
	q := req.Question[0]
	return fmt.Sprintf("%s|%d|%d", strings.ToLower(q.Name), q.Qtype, q.Qclass), true
}

func responseTTL(resp *dns.Msg, maxTTL time.Duration) time.Duration {
	if maxTTL <= 0 {
		maxTTL = 10 * time.Minute
	}
	var ttl uint32
	seen := false
	for _, rr := range allRecords(resp) {
		if rr == nil {
			continue
		}
		recordTTL := rr.Header().Ttl
		if !seen || recordTTL < ttl {
			ttl = recordTTL
			seen = true
		}
	}
	if !seen {
		return maxTTL
	}
	duration := time.Duration(ttl) * time.Second
	if duration > maxTTL {
		return maxTTL
	}
	return duration
}

func capResponseTTL(resp *dns.Msg, remaining uint32) {
	for _, rr := range allRecords(resp) {
		if rr != nil && rr.Header().Ttl > remaining {
			rr.Header().Ttl = remaining
		}
	}
}

func allRecords(resp *dns.Msg) []dns.RR {
	records := make([]dns.RR, 0, len(resp.Answer)+len(resp.Ns)+len(resp.Extra))
	records = append(records, resp.Answer...)
	records = append(records, resp.Ns...)
	records = append(records, resp.Extra...)
	return records
}

func listenPair(addr string) (net.PacketConn, net.Listener, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, nil, "", fmt.Errorf("parse DNS listen address %q: %w", addr, err)
	}

	if port == "0" {
		tcpListener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, nil, "", fmt.Errorf("listen DNS TCP %s: %w", addr, err)
		}
		tcpAddr, ok := tcpListener.Addr().(*net.TCPAddr)
		if !ok {
			_ = tcpListener.Close()
			return nil, nil, "", fmt.Errorf("listen DNS TCP %s: unexpected address %s", addr, tcpListener.Addr())
		}
		actualAddr := net.JoinHostPort(host, strconv.Itoa(tcpAddr.Port))
		udpConn, err := net.ListenPacket("udp", actualAddr)
		if err != nil {
			_ = tcpListener.Close()
			return nil, nil, "", fmt.Errorf("listen DNS UDP %s: %w", actualAddr, err)
		}
		return udpConn, tcpListener, actualAddr, nil
	}

	udpConn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, nil, "", fmt.Errorf("listen DNS UDP %s: %w", addr, err)
	}
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		_ = udpConn.Close()
		return nil, nil, "", fmt.Errorf("listen DNS TCP %s: %w", addr, err)
	}
	return udpConn, tcpListener, addr, nil
}

type RawForwarder struct {
	mode       string
	servers    []string
	timeout    time.Duration
	httpClient *http.Client
}

func NewRawForwarder(cfg resolver.Config) (*RawForwarder, error) {
	mode := cfg.Mode
	servers := append([]string(nil), cfg.Servers...)
	if mode == "" || mode == config.ResolverSystem {
		mode = config.ResolverDoH
		servers = config.DefaultDoHServers()
	}
	if mode == config.ResolverDoH && len(servers) == 0 {
		servers = config.DefaultDoHServers()
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("dns intercept forwarder requires resolver servers for mode %q", mode)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	for i := range servers {
		servers[i] = strings.TrimSpace(servers[i])
		if mode == config.ResolverUDP || mode == config.ResolverTCP {
			servers[i] = withDefaultDNSPort(servers[i])
		}
	}
	return &RawForwarder{
		mode:       mode,
		servers:    servers,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (f *RawForwarder) Forward(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	var errs []error
	for _, server := range f.servers {
		resp, err := f.forwardOne(ctx, server, req)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", server, err))
	}
	return nil, errors.Join(errs...)
}

func (f *RawForwarder) forwardOne(ctx context.Context, server string, req *dns.Msg) (*dns.Msg, error) {
	switch f.mode {
	case config.ResolverUDP, config.ResolverTCP:
		return f.forwardDNS(ctx, server, req)
	case config.ResolverDoH:
		return f.forwardDoH(ctx, server, req)
	default:
		return nil, fmt.Errorf("unsupported resolver mode %q", f.mode)
	}
}

func (f *RawForwarder) forwardDNS(ctx context.Context, server string, req *dns.Msg) (*dns.Msg, error) {
	netMode := "udp"
	if f.mode == config.ResolverTCP {
		netMode = "tcp"
	}
	client := &dns.Client{Net: netMode, Timeout: f.timeout}
	resp, _, err := client.ExchangeContext(ctx, req.Copy(), server)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (f *RawForwarder) forwardDoH(ctx context.Context, server string, req *dns.Msg) (*dns.Msg, error) {
	wire, err := req.Copy().Pack()
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(wire))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/dns-message")
	httpReq.Header.Set("Content-Type", "application/dns-message")

	httpResp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("DoH status %s", httpResp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 65536))
	if err != nil {
		return nil, err
	}
	resp := new(dns.Msg)
	if err := resp.Unpack(data); err != nil {
		return nil, err
	}
	return resp, nil
}

func withDefaultDNSPort(server string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, "53")
}

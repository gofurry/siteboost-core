package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Config struct {
	ListenAddr        string
	NonSteamBehavior  string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type Server struct {
	cfg     Config
	matcher *rules.Matcher
	dialer  Dialer
	logger  *slog.Logger

	mu        sync.Mutex
	server    *http.Server
	transport *http.Transport
	listener  net.Listener
	addr      string
	done      chan error

	activeConns atomic.Int64
}

func New(cfg Config, matcher *rules.Matcher, dialer Dialer, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		cfg:     cfg,
		matcher: matcher,
		dialer:  dialer,
		logger:  logger,
	}
}

func ConfigFromApp(cfg config.Config) Config {
	return Config{
		ListenAddr:        cfg.Proxy.ListenAddr,
		NonSteamBehavior:  cfg.Proxy.NonSteamBehavior,
		ReadHeaderTimeout: cfg.Proxy.ReadHeaderTimeout.Std(),
		IdleTimeout:       cfg.Proxy.IdleTimeout.Std(),
		ShutdownTimeout:   cfg.Proxy.ShutdownTimeout.Std(),
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return fmt.Errorf("proxy server already started")
	}
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.ListenAddr, err)
	}

	s.transport = &http.Transport{
		Proxy:               nil,
		DialContext:         s.dialer.DialContext,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	s.server = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	s.listener = ln
	s.addr = ln.Addr().String()
	s.done = make(chan error, 1)

	go func() {
		err := s.server.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		s.done <- err
		close(s.done)
	}()

	s.logger.Info("proxy started", "addr", s.addr)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.server
	transport := s.transport
	done := s.done
	s.server = nil
	s.transport = nil
	s.listener = nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	err := srv.Shutdown(ctx)
	if err != nil {
		_ = srv.Close()
	}
	if transport != nil {
		transport.CloseIdleConnections()
	}
	if done != nil {
		select {
		case serveErr := <-done:
			if err == nil {
				err = serveErr
			}
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
		}
	}
	s.logger.Info("proxy stopped")
	return err
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) ActiveConns() int64 {
	return s.activeConns.Load()
}

func (s *Server) Wait() <-chan error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.activeConns.Add(1)
	defer s.activeConns.Add(-1)

	if req.Method == http.MethodConnect {
		s.handleConnect(w, req)
		return
	}
	s.handleHTTP(w, req)
}

func (s *Server) handleHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL == nil || req.URL.Host == "" || req.URL.Scheme == "" {
		http.Error(w, "proxy requests must use an absolute URL", http.StatusBadRequest)
		return
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		http.Error(w, "unsupported proxy request scheme", http.StatusBadRequest)
		return
	}

	allowed, match, host := s.matchRequestHost(req.URL.Host)
	if !allowed && s.cfg.NonSteamBehavior == config.NonSteamReject {
		s.logRequest("http_reject", req.Method, host, match, "reject", http.StatusForbidden)
		http.Error(w, "host is not allowed by Steam rules", http.StatusForbidden)
		return
	}

	outReq := req.Clone(req.Context())
	outReq.RequestURI = ""
	removeHopByHopHeaders(outReq.Header)

	resp, err := s.transport.RoundTrip(outReq)
	if err != nil {
		s.logRequest("http_error", req.Method, host, match, s.behaviorFor(allowed), http.StatusBadGateway)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	removeHopByHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)

	s.logRequest("http", req.Method, host, match, s.behaviorFor(allowed), resp.StatusCode)
}

func (s *Server) handleConnect(w http.ResponseWriter, req *http.Request) {
	allowed, match, host := s.matchRequestHost(req.Host)
	if !allowed && s.cfg.NonSteamBehavior == config.NonSteamReject {
		s.logRequest("connect_reject", req.Method, host, match, "reject", http.StatusForbidden)
		http.Error(w, "host is not allowed by Steam rules", http.StatusForbidden)
		return
	}

	target, err := addressWithDefaultPort(req.Host, "443")
	if err != nil {
		http.Error(w, "invalid CONNECT target", http.StatusBadRequest)
		return
	}

	upstreamConn, err := s.dialer.DialContext(req.Context(), "tcp", target)
	if err != nil {
		s.logRequest("connect_error", req.Method, host, match, s.behaviorFor(allowed), http.StatusBadGateway)
		http.Error(w, "upstream dial failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstreamConn.Close()
		http.Error(w, "hijacking is not supported", http.StatusInternalServerError)
		return
	}
	clientConn, rw, err := hijacker.Hijack()
	if err != nil {
		_ = upstreamConn.Close()
		return
	}
	defer clientConn.Close()
	defer upstreamConn.Close()

	if rw.Reader.Buffered() > 0 {
		_ = clientConn.Close()
		return
	}
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}

	s.logRequest("connect", req.Method, host, match, s.behaviorFor(allowed), http.StatusOK)
	tunnel(clientConn, upstreamConn)
}

func (s *Server) matchRequestHost(rawHost string) (bool, rules.MatchResult, string) {
	normalized, err := rules.NormalizeHost(rawHost)
	if err != nil {
		return false, rules.MatchResult{}, rawHost
	}
	match, ok := s.matcher.MatchHost(normalized)
	return ok, match, normalized
}

func (s *Server) behaviorFor(allowed bool) string {
	if allowed {
		return "steam"
	}
	return s.cfg.NonSteamBehavior
}

func (s *Server) logRequest(event, method, host string, match rules.MatchResult, behavior string, status int) {
	attrs := []any{
		"event", event,
		"method", method,
		"host", host,
		"behavior", behavior,
		"status", status,
	}
	if match.GroupName != "" {
		attrs = append(attrs, "rule_group", match.GroupName, "rule", match.Rule)
	}
	s.logger.Info("proxy request", attrs...)
}

func addressWithDefaultPort(raw, defaultPort string) (string, error) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		normalized, normalizeErr := rules.NormalizeHost(raw)
		if normalizeErr != nil {
			return "", normalizeErr
		}
		return net.JoinHostPort(normalized, defaultPort), nil
	}
	normalized, err := rules.NormalizeHost(host)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(normalized, port), nil
}

func tunnel(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go copyAndCloseWrite(b, a, done)
	go copyAndCloseWrite(a, b, done)
	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

func copyAndCloseWrite(dst, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	closeWrite(dst)
	done <- struct{}{}
}

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
		return
	}
	_ = conn.Close()
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopByHopHeaders(header http.Header) {
	for _, token := range header.Values("Connection") {
		for _, part := range strings.Split(token, ",") {
			if name := strings.TrimSpace(part); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Proxy-Connection",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		header.Del(name)
	}
}

func ProxyURL(addr string) *url.URL {
	return &url.URL{Scheme: "http", Host: addr}
}

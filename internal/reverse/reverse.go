package reverse

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
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

type TLSDialer interface {
	DialTLSContext(ctx context.Context, network, address string, tlsConfig *tls.Config) (net.Conn, error)
}

type CertificateProvider interface {
	Certificate(host string) (*tls.Certificate, error)
}

type Config struct {
	HTTPListenAddr    string
	HTTPSListenAddr   string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	RootCAs           *x509.CertPool
}

type Server struct {
	cfg       Config
	matcher   *rules.Matcher
	dialer    Dialer
	certs     CertificateProvider
	logger    *slog.Logger
	transport *http.Transport

	mu        sync.Mutex
	httpSrv   *http.Server
	httpsSrv  *http.Server
	httpAddr  string
	httpsAddr string
	done      []chan error

	activeConns atomic.Int64
}

func ConfigFromApp(cfg config.Config) Config {
	return Config{
		HTTPListenAddr:    cfg.Hosts.HTTPListenAddr,
		HTTPSListenAddr:   cfg.Hosts.HTTPSListenAddr,
		ReadHeaderTimeout: cfg.Proxy.ReadHeaderTimeout.Std(),
		IdleTimeout:       cfg.Proxy.IdleTimeout.Std(),
		ShutdownTimeout:   cfg.Proxy.ShutdownTimeout.Std(),
	}
}

func New(cfg Config, matcher *rules.Matcher, dialer Dialer, certs CertificateProvider, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		cfg:     cfg,
		matcher: matcher,
		dialer:  dialer,
		certs:   certs,
		logger:  logger,
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpSrv != nil || s.httpsSrv != nil {
		return fmt.Errorf("reverse server already started")
	}
	if s.matcher == nil {
		return fmt.Errorf("rules matcher is required")
	}
	if s.dialer == nil {
		return fmt.Errorf("upstream dialer is required")
	}
	if s.certs == nil {
		return fmt.Errorf("certificate provider is required")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    s.cfg.RootCAs,
	}
	s.transport = &http.Transport{
		Proxy:               nil,
		DialContext:         s.dialer.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
	}
	if tlsDialer, ok := s.dialer.(TLSDialer); ok {
		s.transport.DialTLSContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return tlsDialer.DialTLSContext(ctx, network, address, tlsConfig)
		}
	}

	httpLn, err := net.Listen("tcp", s.cfg.HTTPListenAddr)
	if err != nil {
		return fmt.Errorf("listen reverse HTTP %s: %w", s.cfg.HTTPListenAddr, err)
	}
	httpsLn, err := net.Listen("tcp", s.cfg.HTTPSListenAddr)
	if err != nil {
		_ = httpLn.Close()
		return fmt.Errorf("listen reverse HTTPS %s: %w", s.cfg.HTTPSListenAddr, err)
	}

	s.httpSrv = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	tlsLn := tls.NewListener(httpsLn, &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: s.getCertificate,
	})
	s.httpsSrv = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	s.httpAddr = httpLn.Addr().String()
	s.httpsAddr = httpsLn.Addr().String()
	s.done = []chan error{make(chan error, 1), make(chan error, 1)}

	httpSrv := s.httpSrv
	httpsSrv := s.httpsSrv
	httpDone := s.done[0]
	httpsDone := s.done[1]
	go serve(httpSrv, httpLn, httpDone)
	go serve(httpsSrv, tlsLn, httpsDone)

	s.logger.Info("reverse started", "http_addr", s.httpAddr, "https_addr", s.httpsAddr)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	httpSrv := s.httpSrv
	httpsSrv := s.httpsSrv
	done := append([]chan error(nil), s.done...)
	transport := s.transport
	s.httpSrv = nil
	s.httpsSrv = nil
	s.transport = nil
	s.done = nil
	s.mu.Unlock()

	if httpSrv == nil && httpsSrv == nil {
		return nil
	}

	var err error
	if httpSrv != nil {
		if stopErr := httpSrv.Shutdown(ctx); stopErr != nil {
			_ = httpSrv.Close()
			err = stopErr
		}
	}
	if httpsSrv != nil {
		if stopErr := httpsSrv.Shutdown(ctx); err == nil && stopErr != nil {
			_ = httpsSrv.Close()
			err = stopErr
		}
	}
	if transport != nil {
		transport.CloseIdleConnections()
	}
	for _, ch := range done {
		select {
		case serveErr := <-ch:
			if err == nil {
				err = serveErr
			}
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
		}
	}
	s.logger.Info("reverse stopped")
	return err
}

func (s *Server) HTTPAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.httpAddr
}

func (s *Server) HTTPSAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.httpsAddr
}

func (s *Server) ActiveConns() int64 {
	return s.activeConns.Load()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.activeConns.Add(1)
	defer s.activeConns.Add(-1)

	host, match, ok := s.matchRequestHost(req)
	if !ok {
		s.logRequest("reverse_reject", req.Method, host, match, statusForbidden(req))
		http.Error(w, "host is not allowed by Steam rules", http.StatusForbidden)
		return
	}
	scheme := "http"
	defaultPort := "80"
	if req.TLS != nil {
		scheme = "https"
		defaultPort = "443"
	}
	targetAddr := net.JoinHostPort(host, defaultPort)
	proxy := &httputil.ReverseProxy{
		Director: func(out *http.Request) {
			out.URL.Scheme = scheme
			out.URL.Host = targetAddr
			out.Host = host
			out.RequestURI = ""
			out.Header.Del("Proxy-Authorization")
			out.Header.Del("Proxy-Connection")
		},
		Transport: s.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			msg := upstreamErrorMessage(err)
			s.logRequestError("reverse_error", r.Method, host, match, http.StatusBadGateway, msg)
			http.Error(w, "upstream request failed: "+msg, http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, req)
	s.logRequest("reverse", req.Method, host, match, 0)
}

func (s *Server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host, err := rules.NormalizeHost(hello.ServerName)
	if err != nil {
		return nil, err
	}
	if _, ok := s.matcher.MatchHost(host); !ok {
		return nil, fmt.Errorf("host %q is not allowed by Steam rules", host)
	}
	return s.certs.Certificate(host)
}

func (s *Server) matchRequestHost(req *http.Request) (string, rules.MatchResult, bool) {
	rawHost := req.Host
	if rawHost == "" && req.URL != nil {
		rawHost = req.URL.Host
	}
	host, err := rules.NormalizeHost(rawHost)
	if err != nil {
		return rawHost, rules.MatchResult{}, false
	}
	match, ok := s.matcher.MatchHost(host)
	return host, match, ok
}

func (s *Server) logRequest(event, method, host string, match rules.MatchResult, status int) {
	attrs := []any{
		"event", event,
		"method", method,
		"host", host,
	}
	if status != 0 {
		attrs = append(attrs, "status", status)
	}
	if match.GroupName != "" {
		attrs = append(attrs, "rule_group", match.GroupName, "rule", match.Rule)
	}
	s.logger.Info("reverse request", attrs...)
}

func (s *Server) logRequestError(event, method, host string, match rules.MatchResult, status int, message string) {
	attrs := []any{
		"event", event,
		"method", method,
		"host", host,
		"status", status,
		"error", message,
	}
	if match.GroupName != "" {
		attrs = append(attrs, "rule_group", match.GroupName, "rule", match.Rule)
	}
	s.logger.Info("reverse request", attrs...)
}

func upstreamErrorMessage(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t':
			return ' '
		default:
			return r
		}
	}, msg)
	if len(msg) > 1200 {
		msg = msg[:1200] + "..."
	}
	if msg == "" {
		return "unknown error"
	}
	return msg
}

func serve(srv *http.Server, ln net.Listener, done chan<- error) {
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	done <- err
	close(done)
}

func statusForbidden(*http.Request) int {
	return http.StatusForbidden
}

func ProxyURL(addr string) *url.URL {
	return &url.URL{Scheme: "http", Host: addr}
}

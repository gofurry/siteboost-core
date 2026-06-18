package pac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
)

const path = "/proxy.pac"

type Config struct {
	ListenAddr        string
	ProxyAddr         string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type Server struct {
	cfg    Config
	script string
	logger *slog.Logger

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	addr     string
	url      string
	done     chan error
}

func ConfigFromApp(cfg config.Config, proxyAddr string) Config {
	return Config{
		ListenAddr:        cfg.PAC.ListenAddr,
		ProxyAddr:         proxyAddr,
		ReadHeaderTimeout: cfg.Proxy.ReadHeaderTimeout.Std(),
		IdleTimeout:       cfg.Proxy.IdleTimeout.Std(),
		ShutdownTimeout:   cfg.Proxy.ShutdownTimeout.Std(),
	}
}

func New(cfg Config, compiled rules.CompiledRules, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	script, err := Generate(cfg.ProxyAddr, compiled)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, script: script, logger: logger}, nil
}

func Generate(proxyAddr string, compiled rules.CompiledRules) (string, error) {
	if strings.TrimSpace(proxyAddr) == "" {
		return "", fmt.Errorf("proxy address is required")
	}
	if _, _, err := net.SplitHostPort(proxyAddr); err != nil {
		return "", fmt.Errorf("invalid proxy address: %w", err)
	}
	exact := make(map[string]bool, len(compiled.Exact))
	for _, entry := range compiled.Exact {
		if entry.Host != "" {
			exact[entry.Host] = true
		}
	}
	wildcards := make([]string, 0, len(compiled.Wildcard))
	for _, entry := range compiled.Wildcard {
		if entry.Host != "" {
			wildcards = append(wildcards, entry.Host)
		}
	}
	exactJSON, err := json.Marshal(exact)
	if err != nil {
		return "", err
	}
	wildcardJSON, err := json.Marshal(wildcards)
	if err != nil {
		return "", err
	}
	proxyJSON, err := json.Marshal("PROXY " + proxyAddr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`function FindProxyForURL(url, host) {
  host = (host || "").toLowerCase();
  if (host.charAt(host.length - 1) === ".") {
    host = host.substring(0, host.length - 1);
  }

  var exact = %s;
  if (exact[host]) {
    return %s;
  }

  var wildcard = %s;
  for (var i = 0; i < wildcard.length; i++) {
    var suffix = "." + wildcard[i];
    if (host.length > suffix.length && host.substring(host.length - suffix.length) === suffix) {
      return %s;
    }
  }

  return "DIRECT";
}
`, exactJSON, proxyJSON, wildcardJSON, proxyJSON), nil
}

func HostMatches(compiled rules.CompiledRules, rawHost string) bool {
	host, err := rules.NormalizeHost(rawHost)
	if err != nil {
		return false
	}
	for _, entry := range compiled.Exact {
		if host == entry.Host {
			return true
		}
	}
	for _, entry := range compiled.Wildcard {
		if host != entry.Host && strings.HasSuffix(host, "."+entry.Host) {
			return true
		}
	}
	return false
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return fmt.Errorf("pac server already started")
	}
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen PAC %s: %w", s.cfg.ListenAddr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handlePAC)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	srv := s.server
	s.listener = ln
	s.addr = ln.Addr().String()
	s.url = "http://" + s.addr + path
	s.done = make(chan error, 1)

	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		s.done <- err
		close(s.done)
	}()

	s.logger.Info("pac server started", "addr", s.addr, "url", s.url)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.server
	done := s.done
	s.server = nil
	s.listener = nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	err := srv.Shutdown(ctx)
	if err != nil {
		_ = srv.Close()
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
	s.logger.Info("pac server stopped")
	return err
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url
}

func (s *Server) handlePAC(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, s.script)
}

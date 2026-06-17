package runtimecontrol

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type State struct {
	PID        int       `json:"pid"`
	Mode       string    `json:"mode"`
	ProxyAddr  string    `json:"proxy_addr"`
	ControlURL string    `json:"control_url"`
	Token      string    `json:"token"`
	StartedAt  time.Time `json:"started_at"`
}

func GenerateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func WriteState(path string, state State) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("state path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

func ReadState(path string) (State, error) {
	var state State
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parse state: %w", err)
	}
	return state, nil
}

func RemoveState(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type ControlServer struct {
	addr     string
	token    string
	statusFn func() any
	stopFn   func()

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	url      string
	done     chan error
}

func NewControlServer(addr, token string, statusFn func() any, stopFn func()) (*ControlServer, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("control token is required")
	}
	if statusFn == nil {
		return nil, fmt.Errorf("status function is required")
	}
	if stopFn == nil {
		return nil, fmt.Errorf("stop function is required")
	}
	return &ControlServer{
		addr:     addr,
		token:    token,
		statusFn: statusFn,
		stopFn:   stopFn,
	}, nil
}

func (s *ControlServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return fmt.Errorf("control server already started")
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen control %s: %w", s.addr, err)
	}
	if !isLoopbackAddr(ln.Addr().String()) {
		_ = ln.Close()
		return fmt.Errorf("control listener must be loopback")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.withAuth(s.handleStatus))
	mux.HandleFunc("/stop", s.withAuth(s.handleStop))

	s.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	s.listener = ln
	s.url = "http://" + ln.Addr().String()
	s.done = make(chan error, 1)

	go func() {
		err := s.server.Serve(ln)
		if err == http.ErrServerClosed {
			err = nil
		}
		s.done <- err
		close(s.done)
	}()
	return nil
}

func (s *ControlServer) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url
}

func (s *ControlServer) Stop(ctx context.Context) error {
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
	return err
}

func (s *ControlServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, req)
	}
}

func (s *ControlServer) handleStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.statusFn())
}

func (s *ControlServer) handleStop(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("stopping\n"))
	go s.stopFn()
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

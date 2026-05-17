package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type NetworkConfig struct {
	ProxyURL string `json:"proxyUrl"`
}

type NetworkConfigStore struct {
	path string
	mu   sync.Mutex
}

func NewNetworkConfigStore() (*NetworkConfigStore, error) {
	path, err := networkConfigPath()
	if err != nil {
		return nil, err
	}
	return &NetworkConfigStore{path: path}, nil
}

func (s *NetworkConfigStore) Load() (NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return NetworkConfig{}, nil
	}
	if err != nil {
		return NetworkConfig{}, err
	}
	if len(content) == 0 {
		return NetworkConfig{}, nil
	}
	var cfg NetworkConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return NetworkConfig{}, err
	}
	cfg.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	return cfg, nil
}

func (s *NetworkConfigStore) Save(cfg NetworkConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	if err := validateProxyURL(cfg.ProxyURL); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0o600)
}

func NewHTTPClient(proxyAddr string) (*http.Client, error) {
	proxyURL, err := normalizeProxyURL(proxyAddr)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL == nil {
		transport.Proxy = http.ProxyFromEnvironment
	} else {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
	}, nil
}

func validateProxyURL(value string) error {
	_, err := normalizeProxyURL(value)
	return err
}

func normalizeProxyURL(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		return nil, errors.New("proxy scheme must be http, https, or socks5")
	}
	if parsed.Host == "" {
		return nil, errors.New("proxy host is required")
	}
	return parsed, nil
}

func networkConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "SteamScope", "steam-auth-web-session-poc", "network.json"), nil
}

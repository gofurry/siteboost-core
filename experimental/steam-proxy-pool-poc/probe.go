package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const userAgent = "steamscope-network-diagnosis-poc/0.1"

type ProbeConfig struct {
	HTTPTimeout time.Duration
	DialTimeout time.Duration
}

func DefaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		HTTPTimeout: 10 * time.Second,
		DialTimeout: 5 * time.Second,
	}
}

func checkDNS(ctx context.Context, host string) ConnectivityCheck {
	start := time.Now()
	resolver := net.Resolver{}
	_, err := resolver.LookupHost(ctx, host)
	return checkResult("DNS", host, start, err, 0, "")
}

func checkTCP(ctx context.Context, address string, timeout time.Duration) ConnectivityCheck {
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err == nil {
		_ = conn.Close()
	}
	return checkResult("TCP 443", address, start, err, 0, "")
}

func checkTLS(ctx context.Context, address string, timeout time.Duration) ConnectivityCheck {
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{ServerName: "store.steampowered.com", MinVersion: tls.VersionTLS12})
	if err == nil {
		_ = conn.Close()
	}
	return checkResult("TLS handshake", address, start, err, 0, "")
}

func checkPortOpen(ctx context.Context, address string, timeout time.Duration) ConnectivityCheck {
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err == nil {
		_ = conn.Close()
	}
	return checkResult("Port open", address, start, err, 0, "")
}

func checkHTTP(ctx context.Context, client *http.Client, name string, target string, allowLoginRedirect bool) ConnectivityCheck {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return checkResult(name, target, start, err, 0, "")
	}
	req.Header.Set("Accept", "application/json,text/html,*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return checkResult(name, target, start, err, 0, "")
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if readErr != nil {
		return checkResult(name, target, start, readErr, resp.StatusCode, "")
	}
	if allowLoginRedirect && loginRequired(resp, body) {
		return checkResult(name, target, start, nil, resp.StatusCode, "requires_login")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return checkResult(name, target, start, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode), resp.StatusCode, "")
	}
	if !steamHTTPResponseLooksOK(target, body) {
		return checkResult(name, target, start, fmt.Errorf("response is not a valid Steam response"), resp.StatusCode, "")
	}
	return checkResult(name, target, start, nil, resp.StatusCode, "")
}

func checkResult(name, target string, start time.Time, err error, status int, note string) ConnectivityCheck {
	result := ConnectivityCheck{
		Name:       name,
		Target:     target,
		OK:         err == nil,
		DurationMS: time.Since(start).Milliseconds(),
		HTTPStatus: status,
		Note:       note,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func steamHTTPResponseLooksOK(target string, body []byte) bool {
	if strings.Contains(target, "/api/appdetails") {
		var decoded map[string]struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil {
			return false
		}
		for _, item := range decoded {
			if item.Success {
				return true
			}
		}
		return false
	}
	if strings.Contains(target, "api.steampowered.com") {
		return json.Valid(body)
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "steam") || strings.Contains(lower, "steampowered")
}

func loginRequired(resp *http.Response, body []byte) bool {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		if strings.Contains(strings.ToLower(resp.Request.URL.Path), "login") {
			return true
		}
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "login") && (strings.Contains(lower, "steam") || strings.Contains(lower, "sign in"))
}

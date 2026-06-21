package upstream

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultProbeTimeout = 5 * time.Second

type ProbeTarget struct {
	Host string
	Port string
	Path string
}

type ProbeOptions struct {
	Timeout   time.Duration
	TLSConfig *tls.Config
}

type ProbeResult struct {
	Host           string `json:"host"`
	Target         string `json:"target,omitempty"`
	TLSServerName  string `json:"tls_server_name,omitempty"`
	OK             bool   `json:"ok"`
	Stage          string `json:"stage"`
	HTTPStatus     string `json:"http_status,omitempty"`
	Error          string `json:"error,omitempty"`
	DurationMillis int64  `json:"duration_ms"`
}

func DefaultSteamProbeTargets() []ProbeTarget {
	return []ProbeTarget{
		{Host: "steamcommunity.com", Port: "443", Path: "/"},
		{Host: "store.steampowered.com", Port: "443", Path: "/"},
		{Host: "help.steampowered.com", Port: "443", Path: "/"},
		{Host: "media.steampowered.com", Port: "443", Path: "/"},
		{Host: "community.steamstatic.com", Port: "443", Path: "/"},
		{Host: "steamcdn-a.akamaihd.net", Port: "443", Path: "/"},
	}
}

func (d *DirectDialer) ProbeHTTPS(ctx context.Context, targets []ProbeTarget, opts ProbeOptions) []ProbeResult {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultProbeTimeout
	}
	results := make([]ProbeResult, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		i, target := i, normalizeProbeTarget(target)
		wg.Add(1)
		go func() {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
			results[i] = d.probeHTTPS(probeCtx, target, opts)
		}()
	}
	wg.Wait()
	return results
}

func (d *DirectDialer) probeHTTPS(ctx context.Context, target ProbeTarget, opts ProbeOptions) (result ProbeResult) {
	start := time.Now()
	result = ProbeResult{
		Host:  target.Host,
		Stage: "resolve",
	}
	defer func() {
		result.DurationMillis = time.Since(start).Milliseconds()
	}()

	candidates, attempts, err := d.candidates(ctx, target.Host, target.Port)
	if err != nil {
		result.Error = err.Error()
		return
	}
	if len(candidates) == 0 {
		result.Error = probeAttemptsError(attempts)
		if result.Error == "" {
			result.Error = "no probe candidates"
		}
		return
	}

	var failures []string
	for _, attempt := range attempts {
		failures = appendProbeFailure(failures, attempt.Error())
	}
	for _, candidate := range candidates {
		result.Target = candidate.target
		result.TLSServerName = candidate.tlsServerName
		result.Stage = "tcp"
		conn, err := d.dialer.DialContext(ctx, "tcp", candidate.address)
		if err != nil {
			failures = appendProbeFailure(failures, DirectDialAttempt{
				Stage:   "tcp",
				IP:      cloneIP(candidate.ip),
				Address: candidate.address,
				Target:  candidate.target,
				Err:     err,
			}.Error())
			continue
		}

		result.Stage = "tls"
		tlsConn := tls.Client(conn, cloneTLSConfig(opts.TLSConfig, candidate.tlsServerName, target.Host, candidate.ignoreTLSNameMismatch))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = tlsConn.Close()
			failures = appendProbeFailure(failures, DirectDialAttempt{
				Stage:   "tls",
				IP:      cloneIP(candidate.ip),
				Address: candidate.address,
				Target:  candidate.target,
				Err:     err,
			}.Error())
			continue
		}

		result.Stage = "http"
		status, err := probeHTTPHead(ctx, tlsConn, target)
		_ = tlsConn.Close()
		if err != nil {
			failures = appendProbeFailure(failures, fmt.Sprintf("http %s failed: %v", net.JoinHostPort(target.Host, target.Port), err))
			continue
		}
		result.OK = true
		result.HTTPStatus = status
		result.Error = ""
		return
	}

	result.Error = strings.Join(failures, "; ")
	if result.Error == "" {
		result.Error = probeAttemptsError(attempts)
	}
	if result.Error == "" {
		result.Error = "all probe candidates failed"
	}
	return
}

func normalizeProbeTarget(target ProbeTarget) ProbeTarget {
	target.Host = strings.TrimSpace(target.Host)
	target.Port = strings.TrimSpace(target.Port)
	target.Path = strings.TrimSpace(target.Path)
	if target.Port == "" {
		target.Port = "443"
	}
	if target.Path == "" || !strings.HasPrefix(target.Path, "/") {
		target.Path = "/"
	}
	return target
}

func probeHTTPHead(ctx context.Context, conn net.Conn, target ProbeTarget) (string, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return "", err
		}
	}
	req := &http.Request{
		Method: http.MethodHead,
		URL: &url.URL{
			Scheme: "https",
			Host:   target.Host,
			Path:   target.Path,
		},
		Host:   target.Host,
		Header: make(http.Header),
	}
	req.Header.Set("User-Agent", "steam-accelerator-core-probe")
	if err := req.Write(conn); err != nil {
		return "", err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Status, nil
}

func probeAttemptsError(attempts []DirectDialAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	failures := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		failures = appendProbeFailure(failures, attempt.Error())
	}
	return strings.Join(failures, "; ")
}

func appendProbeFailure(failures []string, failure string) []string {
	failure = strings.TrimSpace(failure)
	if failure == "" {
		return failures
	}
	if len(failures) >= 3 {
		return failures
	}
	return append(failures, failure)
}

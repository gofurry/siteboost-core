package reverse

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

type mapDialer map[string]string

func (d mapDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	target, ok := d[address]
	if !ok {
		return nil, fmt.Errorf("unexpected dial target %s", address)
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, target)
}

type errorDialer struct {
	err error
}

func (d errorDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, d.err
}

func TestHTTPReversePreservesHost(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != "store.steampowered.com" {
			t.Fatalf("host = %q", req.Host)
		}
		w.Header().Set("X-Origin", "ok")
		_, _ = w.Write([]byte("ok"))
	}))
	defer origin.Close()
	originURL, _ := url.Parse(origin.URL)

	server := newTestServer(t, mapDialer{
		"store.steampowered.com:80": originURL.Host,
	}, nil)
	defer stopServer(t, server)

	req, err := http.NewRequest(http.MethodGet, "http://"+server.HTTPAddr()+"/hello?q=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "store.steampowered.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("status/body = %d/%q", resp.StatusCode, body)
	}
}

func TestReverseUpstreamErrorIncludesDiagnostic(t *testing.T) {
	dialErr := &upstream.DirectDialError{
		Host: "store.steampowered.com",
		Port: "80",
		Attempts: []upstream.DirectDialAttempt{{
			Stage:   "tcp",
			Address: "203.0.113.20:80",
			Target:  "store.steampowered.com",
			Err:     fmt.Errorf("timeout"),
		}},
	}
	var logs strings.Builder
	server := newTestServerWithLogger(t, errorDialer{err: dialErr}, nil, slog.New(slog.NewTextHandler(&logs, nil)))
	defer stopServer(t, server)

	req, err := http.NewRequest(http.MethodGet, "http://"+server.HTTPAddr()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "store.steampowered.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %q", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "direct upstream dial store.steampowered.com:80 failed") {
		t.Fatalf("body = %q", body)
	}
	logText := logs.String()
	for _, want := range []string{"upstream_error_stage=tcp", "upstream_target=store.steampowered.com", "upstream_attempts=1"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("log = %q, want %q", logText, want)
		}
	}
}

func TestHTTPSReverseUsesDynamicCertificateAndSNI(t *testing.T) {
	manager := certstore.NewWithPlatform(certstore.Config{Dir: t.TempDir()}, &fakeCertPlatform{})
	root, err := manager.EnsureRootCA()
	if err != nil {
		t.Fatal(err)
	}
	rootPool := x509.NewCertPool()
	rootPool.AddCert(root)
	originCert, err := manager.Certificate("store.steampowered.com")
	if err != nil {
		t.Fatal(err)
	}
	var gotSNI string
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != "store.steampowered.com" {
			t.Fatalf("host = %q", req.Host)
		}
		_, _ = w.Write([]byte("secure"))
	}))
	origin.TLS = &tls.Config{
		Certificates: []tls.Certificate{*originCert},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			gotSNI = hello.ServerName
			return nil, nil
		},
	}
	origin.StartTLS()
	defer origin.Close()
	originURL, _ := url.Parse(origin.URL)

	server := newTestServerWithManager(t, mapDialer{
		"store.steampowered.com:443": originURL.Host,
	}, rootPool, manager)
	defer stopServer(t, server)

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:    rootPool,
		ServerName: "store.steampowered.com",
		MinVersion: tls.VersionTLS12,
	}}}
	req, err := http.NewRequest(http.MethodGet, "https://"+server.HTTPSAddr()+"/secure", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "store.steampowered.com"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "secure" {
		t.Fatalf("status/body = %d/%q", resp.StatusCode, body)
	}
	if gotSNI != "store.steampowered.com" {
		t.Fatalf("upstream SNI = %q", gotSNI)
	}
}

func TestRejectsNonSteamHost(t *testing.T) {
	server := newTestServer(t, mapDialer{}, nil)
	defer stopServer(t, server)

	req, err := http.NewRequest(http.MethodGet, "http://"+server.HTTPAddr()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestWebSocketUpgradeIsForwarded(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.ToLower(req.Header.Get("Upgrade")) != "websocket" {
			t.Fatalf("upgrade header = %q", req.Header.Get("Upgrade"))
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("origin does not support hijack")
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		if err := rw.Flush(); err != nil {
			return
		}
		line, _ := rw.Reader.ReadString('\n')
		_, _ = rw.WriteString(line)
		_ = rw.Flush()
	}))
	defer origin.Close()
	originURL, _ := url.Parse(origin.URL)

	server := newTestServer(t, mapDialer{
		"store.steampowered.com:80": originURL.Host,
	}, nil)
	defer stopServer(t, server)

	conn, err := net.Dial("tcp", server.HTTPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: store.steampowered.com\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status line = %q", status)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	_, _ = conn.Write([]byte("ping\n"))
	echo, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if echo != "ping\n" {
		t.Fatalf("echo = %q", echo)
	}
}

func newTestServer(t *testing.T, dialer Dialer, roots *x509.CertPool) *Server {
	t.Helper()
	manager := certstore.NewWithPlatform(certstore.Config{Dir: t.TempDir()}, &fakeCertPlatform{})
	return newTestServerWithManager(t, dialer, roots, manager)
}

func newTestServerWithManager(t *testing.T, dialer Dialer, roots *x509.CertPool, manager *certstore.Manager) *Server {
	t.Helper()
	return newTestServerWithLoggerAndManager(t, dialer, roots, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestServerWithLogger(t *testing.T, dialer Dialer, roots *x509.CertPool, logger *slog.Logger) *Server {
	t.Helper()
	manager := certstore.NewWithPlatform(certstore.Config{Dir: t.TempDir()}, &fakeCertPlatform{})
	return newTestServerWithLoggerAndManager(t, dialer, roots, manager, logger)
}

func newTestServerWithLoggerAndManager(t *testing.T, dialer Dialer, roots *x509.CertPool, manager *certstore.Manager, logger *slog.Logger) *Server {
	t.Helper()
	matcher, err := rules.NewMatcher(rules.DefaultSteamRules, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		HTTPListenAddr:    "127.0.0.1:0",
		HTTPSListenAddr:   "127.0.0.1:0",
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
		RootCAs:           roots,
	}
	server := New(cfg, matcher, dialer, manager, logger)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	return server
}

func stopServer(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

type fakeCertPlatform struct{}

func (*fakeCertPlatform) Name() string { return "windows" }

func (*fakeCertPlatform) IsInstalled(context.Context, *x509.Certificate, string, string) (bool, error) {
	return true, nil
}

func (*fakeCertPlatform) Install(context.Context, *x509.Certificate, string, string) error {
	return nil
}

func (*fakeCertPlatform) Uninstall(context.Context, *x509.Certificate, string) error {
	return nil
}

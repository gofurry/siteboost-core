package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

type mappedDialer struct {
	target string
}

func (d mappedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, d.target)
}

type failingDialer struct {
	err error
}

func (d failingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, d.err
}

type proxyTestResolver struct {
	ips []net.IP
}

func (r proxyTestResolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	return r.ips, nil
}

func newTestProxy(t *testing.T, behavior string, target string, logw io.Writer) *Server {
	t.Helper()
	return newTestProxyWithDialer(t, behavior, mappedDialer{target: target}, logw)
}

func newTestProxyWithDialer(t *testing.T, behavior string, dialer Dialer, logw io.Writer) *Server {
	t.Helper()
	matcher, err := rules.NewMatcher(nil, []string{"*.steam.test", "steam.test"})
	if err != nil {
		t.Fatal(err)
	}
	server := New(Config{
		ListenAddr:        "127.0.0.1:0",
		NonSteamBehavior:  behavior,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}, matcher, dialer, slog.New(slog.NewTextHandler(logw, nil)))
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})
	return server
}

func TestHTTPProxyForSteamHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != "store.steam.test" {
			t.Errorf("Host = %q", req.Host)
		}
		fmt.Fprint(w, "ok")
	}))
	defer upstream.Close()
	target := strings.TrimPrefix(upstream.URL, "http://")

	var logs bytes.Buffer
	proxy := newTestProxy(t, config.NonSteamReject, target, &logs)

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(ProxyURL(proxy.Addr()))}}
	req, err := http.NewRequest(http.MethodGet, "http://store.steam.test/path?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("status/body = %d/%q", resp.StatusCode, body)
	}
	logText := logs.String()
	for _, secret := range []string{"Cookie", "Authorization", "session=secret", "Bearer secret", "token=secret"} {
		if strings.Contains(logText, secret) {
			t.Fatalf("log leaked %q: %s", secret, logText)
		}
	}
}

func TestHTTPProxyRejectsNonSteamByDefault(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "unexpected")
	}))
	defer upstream.Close()
	target := strings.TrimPrefix(upstream.URL, "http://")
	proxy := newTestProxy(t, config.NonSteamReject, target, io.Discard)

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(ProxyURL(proxy.Addr()))}}
	resp, err := client.Get("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestHTTPProxyUpstreamErrorIncludesDiagnostic(t *testing.T) {
	dialErr := &upstream.DirectDialError{
		Host: "store.steam.test",
		Port: "80",
		Attempts: []upstream.DirectDialAttempt{{
			Stage:   "tcp",
			Address: "203.0.113.10:80",
			Target:  "store.steam.test",
			Err:     fmt.Errorf("timeout"),
		}},
	}
	var logs bytes.Buffer
	proxy := newTestProxyWithDialer(t, config.NonSteamReject, failingDialer{err: dialErr}, &logs)

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(ProxyURL(proxy.Addr()))}}
	resp, err := client.Get("http://store.steam.test/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %q", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "direct upstream dial store.steam.test:80 failed") {
		t.Fatalf("body = %q", body)
	}
	logText := logs.String()
	for _, want := range []string{"upstream_error_stage=tcp", "upstream_target=store.steam.test", "upstream_attempts=1"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("log = %q, want %q", logText, want)
		}
	}
}

func TestHTTPProxyDirectsNonSteamWhenConfigured(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "direct")
	}))
	defer upstream.Close()
	target := strings.TrimPrefix(upstream.URL, "http://")
	proxy := newTestProxy(t, config.NonSteamDirect, target, io.Discard)

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(ProxyURL(proxy.Addr()))}}
	resp, err := client.Get("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "direct" {
		t.Fatalf("status/body = %d/%q", resp.StatusCode, body)
	}
}

func TestConnectProxyForSteamHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		_, _ = conn.Write([]byte("pong"))
	}()

	proxy := newTestProxy(t, config.NonSteamReject, ln.Addr().String(), io.Discard)
	conn, err := net.Dial("tcp", proxy.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "CONNECT api.steam.test:443 HTTP/1.1\r\nHost: api.steam.test:443\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("CONNECT status = %q", status)
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
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "pong" {
		t.Fatalf("tunnel response = %q", got)
	}
	<-done
}

func TestHTTPProxyThroughDirectResolver(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.Host, "store.steam.test") {
			t.Errorf("Host = %q", req.Host)
		}
		fmt.Fprint(w, "resolved")
	}))
	defer upstreamServer.Close()
	upstreamAddr := strings.TrimPrefix(upstreamServer.URL, "http://")
	targetURL := "http://store.steam.test:" + portOf(upstreamAddr) + "/"
	dialer := upstream.NewDirectDialer(proxyTestResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}, 5*time.Second)
	proxy := newTestProxyWithDialer(t, config.NonSteamReject, dialer, io.Discard)

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(ProxyURL(proxy.Addr()))}}
	resp, err := client.Get(targetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "resolved" {
		t.Fatalf("status/body = %d/%q", resp.StatusCode, body)
	}
}

func TestConnectProxyThroughHTTPUpstream(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := httpConnectProxyOnce(t, ln, "api.steam.test:443")

	dialer, err := upstream.NewHTTPDialer(upstream.Config{Address: ln.Addr().String(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	proxy := newTestProxyWithDialer(t, config.NonSteamReject, dialer, io.Discard)
	assertConnectTunnel(t, proxy.Addr(), "api.steam.test:443")
	if got := <-done; got != "ping" {
		t.Fatalf("HTTP upstream got %q", got)
	}
}

func TestConnectProxyThroughSOCKS5Upstream(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := socks5ProxyOnce(t, ln, "api.steam.test")

	dialer, err := upstream.NewSOCKS5Dialer(upstream.Config{Address: ln.Addr().String(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	proxy := newTestProxyWithDialer(t, config.NonSteamReject, dialer, io.Discard)
	assertConnectTunnel(t, proxy.Addr(), "api.steam.test:443")
	if got := <-done; got != "ping" {
		t.Fatalf("SOCKS5 upstream got %q", got)
	}
}

func assertConnectTunnel(t *testing.T, proxyAddr, target string) {
	t.Helper()
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("CONNECT status = %q", status)
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
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "pong" {
		t.Fatalf("tunnel response = %q", got)
	}
}

func httpConnectProxyOnce(t *testing.T, ln net.Listener, wantTarget string) <-chan string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		req, err := http.ReadRequest(reader)
		if err != nil {
			done <- "read request: " + err.Error()
			return
		}
		if req.Method != http.MethodConnect || req.Host != wantTarget {
			done <- fmt.Sprintf("bad request: %s %s", req.Method, req.Host)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			done <- "read tunnel: " + err.Error()
			return
		}
		_, _ = conn.Write([]byte("pong"))
		done <- string(buf)
	}()
	return done
}

func socks5ProxyOnce(t *testing.T, ln net.Listener, wantHost string) <-chan string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		header := make([]byte, 2)
		if _, err := io.ReadFull(reader, header); err != nil {
			done <- err.Error()
			return
		}
		methods := make([]byte, int(header[1]))
		if _, err := io.ReadFull(reader, methods); err != nil {
			done <- err.Error()
			return
		}
		_, _ = conn.Write([]byte{0x05, 0x00})
		req := make([]byte, 5)
		if _, err := io.ReadFull(reader, req); err != nil {
			done <- err.Error()
			return
		}
		if req[3] != 0x03 {
			done <- fmt.Sprintf("bad atyp: %d", req[3])
			return
		}
		host := make([]byte, int(req[4]))
		if _, err := io.ReadFull(reader, host); err != nil {
			done <- err.Error()
			return
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(reader, port); err != nil {
			done <- err.Error()
			return
		}
		if string(host) != wantHost {
			done <- "bad host: " + string(host)
			return
		}
		_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0})
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			done <- err.Error()
			return
		}
		_, _ = conn.Write([]byte("pong"))
		done <- string(buf)
	}()
	return done
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

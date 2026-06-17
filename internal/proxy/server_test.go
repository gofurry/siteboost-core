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
)

type mappedDialer struct {
	target string
}

func (d mappedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, d.target)
}

func newTestProxy(t *testing.T, behavior string, target string, logw io.Writer) *Server {
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
	}, matcher, mappedDialer{target: target}, slog.New(slog.NewTextHandler(logw, nil)))
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

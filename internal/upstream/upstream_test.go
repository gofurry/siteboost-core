package upstream

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
)

type fakeResolver struct {
	ips []net.IP
	err error
}

func (r fakeResolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	return r.ips, r.err
}

type mapResolver struct {
	ips map[string][]net.IP
}

func (r mapResolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	if ips, ok := r.ips[host]; ok {
		return ips, nil
	}
	return nil, fmt.Errorf("unexpected resolve host %s", host)
}

func TestDirectDialerUsesResolver(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := echoOnce(t, ln)

	dialer := NewDirectDialer(fakeResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}, 5*time.Second)
	conn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("example.test", portOf(ln.Addr().String())))
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	<-done
}

func TestDirectDialerReportsResolveFailure(t *testing.T) {
	dialer := NewDirectDialer(fakeResolver{err: fmt.Errorf("dns blocked")}, time.Second)
	_, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"direct upstream resolve example.test:443 failed", "dns blocked"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want %q", msg, want)
		}
	}
}

func TestDirectDialerReportsAttemptFailures(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portOf(ln.Addr().String())
	_ = ln.Close()

	dialer := NewDirectDialer(fakeResolver{ips: []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("127.0.0.2"),
	}}, 200*time.Millisecond)
	_, err = dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("example.test", port))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"direct upstream dial " + net.JoinHostPort("example.test", port) + " failed after",
		net.JoinHostPort("127.0.0.1", port),
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want %q", msg, want)
		}
	}
}

func TestDirectDialerDialTLSContext(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
	defer server.Close()

	dialer := NewDirectDialer(fakeResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}, time.Second)
	conn, err := dialer.DialTLSContext(context.Background(), "tcp", net.JoinHostPort("example.test", portOf(server.Listener.Addr().String())), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestDirectDialerProfileUsesForwardHostAndTLSServerName(t *testing.T) {
	var gotSNI string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
	server.TLS = &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			gotSNI = hello.ServerName
			return nil, nil
		},
	}
	server.StartTLS()
	defer server.Close()

	dialer, err := NewDirectDialerWithProfiles(mapResolver{ips: map[string][]net.IP{
		"steamcommunity-a.akamaihd.net": {net.ParseIP("127.0.0.1")},
		"steamcommunity.com":            {net.ParseIP("127.0.0.2")},
	}}, time.Second, []Profile{{
		MatchDomains:  []string{"steamcommunity.com"},
		ForwardHost:   "steamcommunity-a.akamaihd.net",
		TLSServerName: "steamcommunity-a.akamaihd.net",
	}})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialer.DialTLSContext(
		context.Background(),
		"tcp",
		net.JoinHostPort("steamcommunity.com", portOf(server.Listener.Addr().String())),
		&tls.Config{InsecureSkipVerify: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if gotSNI != "steamcommunity-a.akamaihd.net" {
		t.Fatalf("SNI = %q", gotSNI)
	}
}

func TestConfigFromAppAppendsProviderAndUserProfiles(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeHosts
	cfg.Upstream.Profiles = []config.OutboundProfileConfig{{
		MatchDomains: []string{"user.example"},
		ForwardHost:  "user-cdn.example",
	}}
	providerProfiles := []Profile{{
		MatchDomains: []string{"provider.example"},
		ForwardHost:  "provider-cdn.example",
	}}
	got := ConfigFromApp(cfg, providerProfiles)
	if len(got.Profiles) != 2 {
		t.Fatalf("profiles = %#v", got.Profiles)
	}
	if got.Profiles[0].ForwardHost != "provider-cdn.example" || got.Profiles[1].ForwardHost != "user-cdn.example" {
		t.Fatalf("profiles order = %#v", got.Profiles)
	}
}

func TestDirectDialerProbeHTTPSUsesProfileAndOriginalHost(t *testing.T) {
	sniCh := make(chan string, 1)
	requestCh := make(chan struct {
		method string
		host   string
	}, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCh <- struct {
			method string
			host   string
		}{method: req.Method, host: req.Host}
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			select {
			case sniCh <- hello.ServerName:
			default:
			}
			return nil, nil
		},
	}
	server.StartTLS()
	defer server.Close()

	dialer, err := NewDirectDialerWithProfiles(mapResolver{ips: map[string][]net.IP{
		"steamcommunity-a.akamaihd.net": {net.ParseIP("127.0.0.1")},
		"steamcommunity.com":            {net.ParseIP("127.0.0.2")},
	}}, time.Second, []Profile{{
		MatchDomains:  []string{"steamcommunity.com"},
		ForwardHost:   "steamcommunity-a.akamaihd.net",
		TLSServerName: "steamcommunity-a.akamaihd.net",
	}})
	if err != nil {
		t.Fatal(err)
	}

	results := dialer.ProbeHTTPS(context.Background(), []ProbeTarget{{
		Host: "steamcommunity.com",
		Port: portOf(server.Listener.Addr().String()),
	}}, ProbeOptions{
		Timeout:   time.Second,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if !got.OK {
		t.Fatalf("probe failed: %#v", got)
	}
	if got.Stage != "http" || got.HTTPStatus != "204 No Content" {
		t.Fatalf("probe stage/status = %q/%q", got.Stage, got.HTTPStatus)
	}
	if got.Target != "steamcommunity-a.akamaihd.net" || got.TLSServerName != "steamcommunity-a.akamaihd.net" {
		t.Fatalf("probe target/SNI = %q/%q", got.Target, got.TLSServerName)
	}

	select {
	case gotSNI := <-sniCh:
		if gotSNI != "steamcommunity-a.akamaihd.net" {
			t.Fatalf("SNI = %q", gotSNI)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive TLS ClientHello")
	}
	select {
	case gotReq := <-requestCh:
		if gotReq.method != http.MethodHead || gotReq.host != "steamcommunity.com" {
			t.Fatalf("request = %#v", gotReq)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive HTTP probe")
	}
}

func TestDirectDialerProbeHTTPSReportsResolveFailure(t *testing.T) {
	dialer := NewDirectDialer(fakeResolver{err: fmt.Errorf("doh blocked")}, time.Second)
	results := dialer.ProbeHTTPS(context.Background(), []ProbeTarget{{
		Host: "example.test",
		Port: "443",
	}}, ProbeOptions{Timeout: time.Second})
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if got.OK {
		t.Fatalf("probe unexpectedly succeeded: %#v", got)
	}
	if got.Stage != "resolve" {
		t.Fatalf("stage = %q, want resolve", got.Stage)
	}
	if !strings.Contains(got.Error, "doh blocked") {
		t.Fatalf("error = %q", got.Error)
	}
}

func TestConfigFromAppAllowsNoProviderProfiles(t *testing.T) {
	cfg := config.Default()
	got := ConfigFromApp(cfg, nil)
	if len(got.Profiles) != 0 {
		t.Fatalf("profiles = %#v", got.Profiles)
	}
}

func TestHTTPDialerConnectsWithBasicAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
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
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if req.Method != http.MethodConnect || req.Host != "example.test:443" {
			done <- fmt.Sprintf("bad CONNECT request: %s %s", req.Method, req.Host)
			return
		}
		if req.Header.Get("Proxy-Authorization") != wantAuth {
			done <- "bad auth"
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

	dialer, err := NewHTTPDialer(Config{
		Address:  ln.Addr().String(),
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestSOCKS5DialerNoAuthUsesDomainTarget(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := socks5Once(t, ln, false, "example.test")

	dialer, err := NewSOCKS5Dialer(Config{Address: ln.Addr().String(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestSOCKS5DialerUsernamePassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := socks5Once(t, ln, true, "example.test")

	dialer, err := NewSOCKS5Dialer(Config{
		Address:  ln.Addr().String(),
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestProxyAddressParseErrorDoesNotLeakPassword(t *testing.T) {
	_, err := NewHTTPDialer(Config{
		Address:  "http://user:secret@\n",
		Password: "another-secret",
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	msg := err.Error()
	for _, secret := range []string{"secret", "another-secret"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("error leaked password %q: %s", secret, msg)
		}
	}
}

func echoOnce(t *testing.T, ln net.Listener) <-chan struct{} {
	t.Helper()
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
	return done
}

func socks5Once(t *testing.T, ln net.Listener, auth bool, wantHost string) <-chan string {
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
		if auth {
			_, _ = conn.Write([]byte{0x05, 0x02})
			authReq := make([]byte, 2)
			if _, err := io.ReadFull(reader, authReq); err != nil {
				done <- err.Error()
				return
			}
			username := make([]byte, int(authReq[1]))
			if _, err := io.ReadFull(reader, username); err != nil {
				done <- err.Error()
				return
			}
			passLen, err := reader.ReadByte()
			if err != nil {
				done <- err.Error()
				return
			}
			password := make([]byte, int(passLen))
			if _, err := io.ReadFull(reader, password); err != nil {
				done <- err.Error()
				return
			}
			if string(username) != "user" || string(password) != "pass" {
				done <- "bad credentials"
				return
			}
			_, _ = conn.Write([]byte{0x01, 0x00})
		} else {
			_, _ = conn.Write([]byte{0x05, 0x00})
		}

		req := make([]byte, 5)
		if _, err := io.ReadFull(reader, req); err != nil {
			done <- err.Error()
			return
		}
		if req[0] != 0x05 || req[1] != 0x01 || req[3] != 0x03 {
			done <- fmt.Sprintf("bad request header: %v", req[:4])
			return
		}
		hostLen := int(req[4])
		host := make([]byte, hostLen)
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

func assertTunnel(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "pong" {
		t.Fatalf("got %q, want pong", got)
	}
}

func portOf(addr string) string {
	parts := strings.Split(addr, ":")
	return parts[len(parts)-1]
}

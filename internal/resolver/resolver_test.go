package resolver

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/miekg/dns"
)

func TestSystemResolverLocalhost(t *testing.T) {
	resolver, err := New(Config{
		Mode:       config.ResolverSystem,
		PreferIPv4: true,
		CacheTTL:   time.Minute,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ips, err := resolver.Resolve(context.Background(), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatalf("no localhost IPs returned")
	}
}

func TestUDPResolverCacheAndPreferIPv4(t *testing.T) {
	var queries atomic.Int32
	addr, shutdown := startDNSServer(t, "udp", func(req *dns.Msg) *dns.Msg {
		queries.Add(1)
		return dnsReply(req, "192.0.2.10", "2001:db8::10", 60)
	})
	defer shutdown()

	resolver, err := New(Config{
		Mode:       config.ResolverUDP,
		Servers:    []string{addr},
		PreferIPv4: true,
		CacheTTL:   time.Minute,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ips, err := resolver.Resolve(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if got := ips[0].String(); got != "192.0.2.10" {
		t.Fatalf("first IP = %s", got)
	}
	firstCount := queries.Load()
	if _, err := resolver.Resolve(context.Background(), "example.test"); err != nil {
		t.Fatal(err)
	}
	if queries.Load() != firstCount {
		t.Fatalf("expected cache hit; queries changed from %d to %d", firstCount, queries.Load())
	}
}

func TestTCPResolverDisableIPv6(t *testing.T) {
	addr, shutdown := startDNSServer(t, "tcp", func(req *dns.Msg) *dns.Msg {
		return dnsReply(req, "192.0.2.20", "2001:db8::20", 60)
	})
	defer shutdown()

	resolver, err := New(Config{
		Mode:        config.ResolverTCP,
		Servers:     []string{addr},
		DisableIPv6: true,
		CacheTTL:    time.Minute,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ips, err := resolver.Resolve(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || ips[0].To4() == nil {
		t.Fatalf("ips = %#v, want only IPv4", ips)
	}
}

func TestDoHResolverFallback(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Type") != "application/dns-message" {
			t.Errorf("content-type = %q", req.Header.Get("Content-Type"))
		}
		msg := new(dns.Msg)
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		_ = req.Body.Close()
		if err := msg.Unpack(data); err != nil {
			t.Errorf("unpack request: %v", err)
		}
		wire, err := dnsReply(msg, "192.0.2.30", "", 60).Pack()
		if err != nil {
			t.Errorf("pack response: %v", err)
		}
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(wire)
	}))
	defer good.Close()

	resolver, err := New(Config{
		Mode:        config.ResolverDoH,
		Servers:     []string{bad.URL, good.URL},
		DisableIPv6: true,
		CacheTTL:    time.Minute,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ips, err := resolver.Resolve(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || ips[0].String() != "192.0.2.30" {
		t.Fatalf("ips = %#v", ips)
	}
}

func TestResolverNoUsableIPs(t *testing.T) {
	addr, shutdown := startDNSServer(t, "udp", func(req *dns.Msg) *dns.Msg {
		return dnsReply(req, "", "2001:db8::40", 60)
	})
	defer shutdown()

	resolver, err := New(Config{
		Mode:        config.ResolverUDP,
		Servers:     []string{addr},
		DisableIPv6: true,
		CacheTTL:    time.Minute,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "example.test"); err == nil {
		t.Fatalf("expected no usable IPs error")
	}
}

func startDNSServer(t *testing.T, network string, reply func(*dns.Msg) *dns.Msg) (string, func()) {
	t.Helper()
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		_ = w.WriteMsg(reply(req))
	})
	server := &dns.Server{Net: network, Handler: handler}
	switch network {
	case "udp":
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server.PacketConn = conn
		go func() { _ = server.ActivateAndServe() }()
		return conn.LocalAddr().String(), func() { _ = server.Shutdown() }
	case "tcp":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = ln
		go func() { _ = server.ActivateAndServe() }()
		return ln.Addr().String(), func() { _ = server.Shutdown() }
	default:
		t.Fatalf("unsupported network %s", network)
		return "", nil
	}
}

func dnsReply(req *dns.Msg, a, aaaa string, ttl uint32) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)
	if len(req.Question) == 0 {
		return resp
	}
	name := req.Question[0].Name
	switch req.Question[0].Qtype {
	case dns.TypeA:
		if a != "" {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   net.ParseIP(a),
			})
		}
	case dns.TypeAAAA:
		if aaaa != "" {
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: net.ParseIP(aaaa),
			})
		}
	}
	return resp
}

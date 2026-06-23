package dnsintercept

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/miekg/dns"
)

type fakeForwarder struct {
	fn func(*dns.Msg) (*dns.Msg, error)
}

func (f fakeForwarder) Forward(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	return f.fn(req)
}

func TestTargetAAndAAAARecords(t *testing.T) {
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		t.Fatalf("target query should not be forwarded")
		return nil, nil
	}})

	aResp := server.handle(query("steamcommunity.com.", dns.TypeA))
	if len(aResp.Answer) != 1 {
		t.Fatalf("A answers = %#v", aResp.Answer)
	}
	if got := aResp.Answer[0].(*dns.A).A.String(); got != "127.0.0.1" {
		t.Fatalf("A = %s", got)
	}

	aaaaResp := server.handle(query("steamcommunity.com.", dns.TypeAAAA))
	if len(aaaaResp.Answer) != 1 {
		t.Fatalf("AAAA answers = %#v", aaaaResp.Answer)
	}
	if got := aaaaResp.Answer[0].(*dns.AAAA).AAAA.String(); got != "::1" {
		t.Fatalf("AAAA = %s", got)
	}

	status := server.Status()
	if status.TargetQueries != 2 || status.ForwardedQueries != 0 {
		t.Fatalf("status = %#v", status)
	}
}

func TestTargetHTTPSRecordBlocked(t *testing.T) {
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		t.Fatalf("target HTTPS query should not be forwarded")
		return nil, nil
	}})

	resp := server.handle(query("steamcommunity.com.", dns.TypeHTTPS))
	if resp.Rcode != dns.RcodeSuccess || len(resp.Answer) != 0 {
		t.Fatalf("response = %#v", resp)
	}
	status := server.Status()
	if status.TargetQueries != 1 || status.BlockedQueries != 1 {
		t.Fatalf("status = %#v", status)
	}
}

func TestTargetHTTPSRecordCanBeForwarded(t *testing.T) {
	forwarded := false
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		forwarded = true
		resp := new(dns.Msg)
		resp.SetReply(req)
		return resp, nil
	}})
	server.cfg.BlockHTTPSRecords = false

	resp := server.handle(query("steamcommunity.com.", dns.TypeHTTPS))
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("rcode = %s", dns.RcodeToString[resp.Rcode])
	}
	if !forwarded {
		t.Fatalf("target HTTPS query was not forwarded")
	}
	status := server.Status()
	if status.TargetQueries != 1 || status.ForwardedQueries != 1 || status.BlockedQueries != 0 {
		t.Fatalf("status = %#v", status)
	}
}

func TestNonTargetQueryForwarded(t *testing.T) {
	calls := 0
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		calls++
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.0.2.55"),
		})
		return resp, nil
	}})

	resp := server.handle(query("example.test.", dns.TypeA))
	if len(resp.Answer) != 1 {
		t.Fatalf("answers = %#v", resp.Answer)
	}
	if got := resp.Answer[0].(*dns.A).A.String(); got != "192.0.2.55" {
		t.Fatalf("A = %s", got)
	}
	cachedResp := server.handle(query("example.test.", dns.TypeA))
	if len(cachedResp.Answer) != 1 {
		t.Fatalf("cached answers = %#v", cachedResp.Answer)
	}
	if calls != 1 {
		t.Fatalf("forwarder calls = %d, want 1", calls)
	}
	status := server.Status()
	if status.TargetQueries != 0 || status.ForwardedQueries != 1 || status.CacheHits != 1 || status.ErrorQueries != 0 {
		t.Fatalf("status = %#v", status)
	}
}

func TestForwardErrorReturnsServerFailure(t *testing.T) {
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		return nil, errors.New("upstream failed")
	}})

	resp := server.handle(query("example.test.", dns.TypeA))
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("rcode = %s", dns.RcodeToString[resp.Rcode])
	}
	if server.Status().ErrorQueries != 1 {
		t.Fatalf("status = %#v", server.Status())
	}
}

func TestServerServesUDPAndTCP(t *testing.T) {
	server := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) {
		t.Fatalf("target query should not be forwarded")
		return nil, nil
	}})
	server.cfg.ListenAddr = "127.0.0.1:0"
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})
	addr := server.Status().ListenAddr

	for _, network := range []string{"udp", "tcp"} {
		t.Run(network, func(t *testing.T) {
			resp := exchangeDNS(t, network, addr, query("steamcommunity.com.", dns.TypeA))
			if len(resp.Answer) != 1 || resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
				t.Fatalf("response = %#v", resp.Answer)
			}
		})
	}
}

func TestServerPortConflictReportsListenError(t *testing.T) {
	first := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) { return nil, nil }})
	first.cfg.ListenAddr = "127.0.0.1:0"
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = first.Stop(ctx)
	})

	second := newTestServer(t, fakeForwarder{fn: func(req *dns.Msg) (*dns.Msg, error) { return nil, nil }})
	second.cfg.ListenAddr = first.Status().ListenAddr
	err := second.Start()
	if err == nil {
		t.Fatalf("expected port conflict")
	}
	if !strings.Contains(err.Error(), "listen DNS") {
		t.Fatalf("error = %v", err)
	}
}

func newTestServer(t *testing.T, forwarder Forwarder) *Server {
	t.Helper()
	matcher, err := rules.NewMatcher([]rules.RuleGroup{{
		Name:    "test",
		Domains: []string{"steamcommunity.com", "*.steampowered.com"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		Strategy:          "manual",
		ListenAddr:        "127.0.0.1:0",
		MapIPv4:           net.ParseIP("127.0.0.1").To4(),
		MapIPv6:           net.ParseIP("::1").To16(),
		TTL:               30 * time.Second,
		BlockHTTPSRecords: true,
		ForwardTimeout:    5 * time.Second,
	}, matcher, forwarder)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func query(name string, qtype uint16) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(name, qtype)
	return msg
}

func exchangeDNS(t *testing.T, network, addr string, msg *dns.Msg) *dns.Msg {
	t.Helper()
	client := &dns.Client{Net: network, Timeout: time.Second}
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, _, err := client.Exchange(msg.Copy(), addr)
		if err == nil {
			return resp
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("exchange %s %s: %v", network, addr, lastErr)
	return nil
}

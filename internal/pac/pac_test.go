package pac

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/rules"
)

func TestGeneratePACAndHostMatching(t *testing.T) {
	matcher, err := rules.NewMatcher([]rules.RuleGroup{
		{Name: "steam", Domains: []string{"store.steampowered.com", "*.steamcommunity.com"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	compiled := matcher.Rules()
	script, err := Generate("127.0.0.1:26501", compiled)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"store.steampowered.com":true`,
		`"steamcommunity.com"`,
		`PROXY 127.0.0.1:26501`,
		`return "DIRECT"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("PAC script missing %q:\n%s", want, script)
		}
	}

	tests := []struct {
		host string
		want bool
	}{
		{host: "store.steampowered.com", want: true},
		{host: "a.steamcommunity.com", want: true},
		{host: "steamcommunity.com", want: false},
		{host: "evilsteamcommunity.com", want: false},
	}
	for _, tt := range tests {
		if got := HostMatches(compiled, tt.host); got != tt.want {
			t.Fatalf("HostMatches(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestServerServesPACAndStops(t *testing.T) {
	matcher, err := rules.NewMatcher([]rules.RuleGroup{
		{Name: "steam", Domains: []string{"store.steampowered.com"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		ListenAddr:        "127.0.0.1:0",
		ProxyAddr:         "127.0.0.1:26501",
		ReadHeaderTimeout: time.Second,
		IdleTimeout:       time.Second,
		ShutdownTimeout:   time.Second,
	}, matcher.Rules(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-ns-proxy-autoconfig") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(string(body), "store.steampowered.com") {
		t.Fatalf("body = %s", body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get(server.URL()); err == nil {
		t.Fatalf("expected request to stopped PAC server to fail")
	}
}

package diagnostics

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/upstream"
)

func TestUpstreamErrorMessageSanitizesWhitespaceAndLength(t *testing.T) {
	err := fmt.Errorf("first\nsecond\tthird")
	got := UpstreamErrorMessage(err)
	if got != "first second third" {
		t.Fatalf("message = %q", got)
	}
}

func TestUpstreamErrorAttrsForDirectDialError(t *testing.T) {
	err := &upstream.DirectDialError{
		Host: "steamcommunity.com",
		Port: "443",
		Attempts: []upstream.DirectDialAttempt{{
			Stage:   "tcp",
			IP:      net.ParseIP("203.0.113.1"),
			Address: "203.0.113.1:443",
			Target:  "steamcommunity-a.akamaihd.net",
			Err:     fmt.Errorf("timeout"),
		}},
	}
	got := attrsText(UpstreamErrorAttrs(err))
	for _, want := range []string{
		"upstream_error_type=direct",
		"upstream_host=steamcommunity.com",
		"upstream_port=443",
		"upstream_error_stage=tcp",
		"upstream_attempts=1",
		"upstream_target=steamcommunity-a.akamaihd.net",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("attrs = %q, want %q", got, want)
		}
	}
}

func attrsText(attrs []any) string {
	var b strings.Builder
	for i := 0; i+1 < len(attrs); i += 2 {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("%v=%v", attrs[i], attrs[i+1]))
	}
	return b.String()
}

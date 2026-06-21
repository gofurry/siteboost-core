package diagnostics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofurry/go-steam-core/internal/upstream"
)

func UpstreamErrorMessage(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t':
			return ' '
		default:
			return r
		}
	}, msg)
	if len(msg) > 1200 {
		msg = msg[:1200] + "..."
	}
	if msg == "" {
		return "unknown error"
	}
	return msg
}

func UpstreamErrorAttrs(err error) []any {
	var directErr *upstream.DirectDialError
	if !errors.As(err, &directErr) {
		return nil
	}
	stage := "dial"
	target := ""
	if directErr.ResolveErr != nil {
		stage = "resolve"
		target = fmt.Sprintf("%s:%s", directErr.Host, directErr.Port)
	} else if len(directErr.Attempts) > 0 {
		attempt := directErr.Attempts[len(directErr.Attempts)-1]
		stage = attempt.Stage
		if stage == "" {
			stage = "tcp"
		}
		target = attempt.Address
		if attempt.Target != "" {
			target = attempt.Target
		}
	}
	attrs := []any{
		"upstream_error_type", "direct",
		"upstream_host", directErr.Host,
		"upstream_port", directErr.Port,
		"upstream_error_stage", stage,
		"upstream_attempts", len(directErr.Attempts),
	}
	if target != "" {
		attrs = append(attrs, "upstream_target", target)
	}
	return attrs
}

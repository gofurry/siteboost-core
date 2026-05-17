package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func normalizeCandidate(candidate ProxyCandidate) ProxyCandidate {
	candidate.Address = strings.TrimSpace(candidate.Address)
	candidate.ProxyURL = strings.TrimSpace(candidate.ProxyURL)
	if candidate.Protocol == "" {
		candidate.Protocol = ProxyProtocolUnknown
	}
	if candidate.ProxyURL == "" && candidate.Address != "" {
		switch candidate.Protocol {
		case ProxyProtocolSOCKS5:
			candidate.ProxyURL = "socks5://" + candidate.Address
		case ProxyProtocolHTTPS:
			candidate.ProxyURL = "https://" + candidate.Address
		default:
			candidate.ProxyURL = "http://" + candidate.Address
		}
	}
	if candidate.Address == "" && candidate.ProxyURL != "" {
		if parsed, err := url.Parse(candidate.ProxyURL); err == nil {
			candidate.Address = parsed.Host
		}
	}
	return candidate
}

func normalizeProxyURL(value string, fallback ProxyProtocol) (ProxyCandidate, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ProxyCandidate{}, errors.New("proxy address is required")
	}
	protocol := fallback
	if protocol == "" || protocol == ProxyProtocolMixed || protocol == ProxyProtocolUnknown {
		protocol = ProxyProtocolHTTP
	}
	if !strings.Contains(value, "://") {
		value = string(protocol) + "://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ProxyCandidate{}, err
	}
	if parsed.Host == "" {
		return ProxyCandidate{}, errors.New("proxy host is required")
	}
	switch parsed.Scheme {
	case "http":
		protocol = ProxyProtocolHTTP
	case "https":
		protocol = ProxyProtocolHTTPS
	case "socks5", "socks5h":
		protocol = ProxyProtocolSOCKS5
	default:
		return ProxyCandidate{}, errors.New("proxy scheme must be http, https, or socks5")
	}
	return normalizeCandidate(ProxyCandidate{
		Name:     "Manual Proxy",
		Address:  parsed.Host,
		ProxyURL: parsed.String(),
		Protocol: protocol,
		Source:   "manual",
	}), nil
}

func buildHTTPClient(candidate *ProxyCandidate, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext

	if candidate != nil && candidate.ProxyURL != "" {
		parsed, err := url.Parse(candidate.ProxyURL)
		if err != nil {
			return nil, err
		}
		switch parsed.Scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsed)
		case "socks5", "socks5h":
			dialer, err := proxy.SOCKS5("tcp", parsed.Host, nil, proxy.Direct)
			if err != nil {
				return nil, err
			}
			contextDialer, ok := dialer.(proxy.ContextDialer)
			if ok {
				transport.DialContext = contextDialer.DialContext
			} else {
				transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
					type result struct {
						conn net.Conn
						err  error
					}
					ch := make(chan result, 1)
					go func() {
						conn, err := dialer.Dial(network, address)
						ch <- result{conn: conn, err: err}
					}()
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case res := <-ch:
						return res.conn, res.err
					}
				}
			}
		default:
			return nil, errors.New("unsupported proxy scheme")
		}
	}

	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

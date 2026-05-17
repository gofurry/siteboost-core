package main

import (
	"net/url"
	"os"
	"strings"
)

func DiscoverSystemProxies() SystemProxyInfo {
	info := discoverPlatformSystemProxies()
	envCandidates := discoverEnvironmentProxies()
	if len(envCandidates) > 0 {
		info.Candidates = append(envCandidates, info.Candidates...)
	}
	info.Candidates = dedupeCandidates(info.Candidates)
	return info
}

func discoverEnvironmentProxies() []ProxyCandidate {
	keys := []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"}
	candidates := make([]ProxyCandidate, 0)
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		candidate, err := normalizeProxyURL(value, ProxyProtocolHTTP)
		if err != nil {
			continue
		}
		candidate.Name = "System Proxy (" + key + ")"
		candidate.Source = "system_proxy"
		candidates = append(candidates, candidate)
	}
	return candidates
}

func parseProxyServer(value string) []ProxyCandidate {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ";")
	candidates := make([]ProxyCandidate, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			pair := strings.SplitN(part, "=", 2)
			scheme := strings.ToLower(strings.TrimSpace(pair[0]))
			addr := strings.TrimSpace(pair[1])
			protocol := protocolFromSystemScheme(scheme)
			candidates = append(candidates, normalizeCandidate(ProxyCandidate{
				Name:     "System Proxy (" + scheme + ")",
				Address:  stripProxyScheme(addr),
				Protocol: protocol,
				Source:   "system_proxy",
			}))
			continue
		}
		candidate, err := normalizeProxyURL(part, ProxyProtocolHTTP)
		if err == nil {
			candidate.Name = "System Proxy"
			candidate.Source = "system_proxy"
			candidates = append(candidates, candidate)
		}
	}
	return dedupeCandidates(candidates)
}

func protocolFromSystemScheme(scheme string) ProxyProtocol {
	switch scheme {
	case "socks", "socks5":
		return ProxyProtocolSOCKS5
	case "https":
		return ProxyProtocolHTTPS
	default:
		return ProxyProtocolHTTP
	}
}

func stripProxyScheme(value string) string {
	if !strings.Contains(value, "://") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	return parsed.Host
}

func dedupeCandidates(candidates []ProxyCandidate) []ProxyCandidate {
	out := make([]ProxyCandidate, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizeCandidate(candidate)
		key := string(candidate.Protocol) + "|" + candidate.ProxyURL
		if key == "|" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

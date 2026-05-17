package main

func CommonLocalCandidates() []ProxyCandidate {
	candidates := []ProxyCandidate{
		{Name: "HTTP/Mixed 7890", Address: "127.0.0.1:7890", Protocol: ProxyProtocolMixed, Source: "common_port"},
		{Name: "HTTP/Mixed 7897", Address: "127.0.0.1:7897", Protocol: ProxyProtocolMixed, Source: "common_port"},
		{Name: "HTTP/Mixed 8080", Address: "127.0.0.1:8080", Protocol: ProxyProtocolMixed, Source: "common_port"},
		{Name: "HTTP/Mixed 10809", Address: "127.0.0.1:10809", Protocol: ProxyProtocolMixed, Source: "common_port"},
		{Name: "HTTP/Mixed 20171", Address: "127.0.0.1:20171", Protocol: ProxyProtocolMixed, Source: "common_port"},
		{Name: "SOCKS5 1080", Address: "127.0.0.1:1080", Protocol: ProxyProtocolSOCKS5, Source: "common_port"},
		{Name: "SOCKS5 7891", Address: "127.0.0.1:7891", Protocol: ProxyProtocolSOCKS5, Source: "common_port"},
		{Name: "SOCKS5 10808", Address: "127.0.0.1:10808", Protocol: ProxyProtocolSOCKS5, Source: "common_port"},
	}
	for i := range candidates {
		candidates[i] = normalizeCandidate(candidates[i])
	}
	return candidates
}

package upstream

import (
	"fmt"
	"net"
	"strings"

	"github.com/gofurry/go-steam-core/internal/rules"
)

type Profile struct {
	MatchDomains          []string
	CandidateIPs          []string
	ForwardHost           string
	TLSServerName         string
	IgnoreTLSNameMismatch bool
}

type compiledProfile struct {
	matchDomains          []compiledProfileDomain
	candidateIPs          []net.IP
	forwardHost           string
	tlsServerName         string
	ignoreTLSNameMismatch bool
}

type compiledProfileDomain struct {
	host     string
	wildcard bool
}

func DefaultSteamProfiles() []Profile {
	return []Profile{
		{
			MatchDomains: []string{
				"steamcommunity.com",
				"*.steamcommunity.com",
			},
			ForwardHost:   "steamcommunity-a.akamaihd.net",
			TLSServerName: "steamcommunity-a.akamaihd.net",
		},
		{
			MatchDomains: []string{
				"store.steampowered.com",
				"checkout.steampowered.com",
				"help.steampowered.com",
				"login.steampowered.com",
			},
			ForwardHost:   "cdn-a.akamaihd.net",
			TLSServerName: "cdn-a.akamaihd.net",
		},
	}
}

func compileProfiles(profiles []Profile) ([]compiledProfile, error) {
	if len(profiles) == 0 {
		return nil, nil
	}
	compiled := make([]compiledProfile, 0, len(profiles))
	for i, profile := range profiles {
		next := compiledProfile{
			ignoreTLSNameMismatch: profile.IgnoreTLSNameMismatch,
		}
		for j, domain := range profile.MatchDomains {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}
			wildcard := strings.HasPrefix(domain, "*.")
			if strings.Contains(domain, "*") && !wildcard {
				return nil, fmt.Errorf("profiles[%d] match_domains[%d] must use the *.example.com form", i, j)
			}
			if wildcard {
				domain = strings.TrimPrefix(domain, "*.")
			}
			host, err := rules.NormalizeHost(domain)
			if err != nil {
				return nil, fmt.Errorf("profiles[%d] match_domains[%d]: %w", i, j, err)
			}
			next.matchDomains = append(next.matchDomains, compiledProfileDomain{host: host, wildcard: wildcard})
		}
		if len(next.matchDomains) == 0 {
			return nil, fmt.Errorf("profiles[%d] match_domains is required", i)
		}

		for j, rawIP := range profile.CandidateIPs {
			rawIP = strings.TrimSpace(rawIP)
			if rawIP == "" {
				continue
			}
			ip := net.ParseIP(rawIP)
			if ip == nil {
				return nil, fmt.Errorf("profiles[%d] candidate_ips[%d] must be an IP address", i, j)
			}
			next.candidateIPs = append(next.candidateIPs, cloneIP(ip))
		}

		var err error
		next.forwardHost, err = normalizeProfileHost(profile.ForwardHost)
		if err != nil {
			return nil, fmt.Errorf("profiles[%d] forward_host: %w", i, err)
		}
		next.tlsServerName, err = normalizeProfileHost(profile.TLSServerName)
		if err != nil {
			return nil, fmt.Errorf("profiles[%d] tls_server_name: %w", i, err)
		}
		if len(next.candidateIPs) == 0 &&
			next.forwardHost == "" &&
			next.tlsServerName == "" &&
			!next.ignoreTLSNameMismatch {
			return nil, fmt.Errorf("profiles[%d] must define candidate_ips, forward_host, tls_server_name, or ignore_tls_name_mismatch", i)
		}
		compiled = append(compiled, next)
	}
	return compiled, nil
}

func normalizeProfileHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	return rules.NormalizeHost(host)
}

func (d *DirectDialer) matchProfile(host string) *compiledProfile {
	if len(d.profiles) == 0 {
		return nil
	}
	normalized, err := rules.NormalizeHost(host)
	if err != nil {
		return nil
	}
	for i := range d.profiles {
		profile := &d.profiles[i]
		for _, domain := range profile.matchDomains {
			if domain.wildcard {
				if normalized != domain.host && strings.HasSuffix(normalized, "."+domain.host) {
					return profile
				}
				continue
			}
			if normalized == domain.host {
				return profile
			}
		}
	}
	return nil
}

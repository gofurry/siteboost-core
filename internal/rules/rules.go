package rules

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

type RuleGroup struct {
	Name    string
	Domains []string
}

type MatchResult struct {
	Host      string
	GroupName string
	Rule      string
}

type CompiledRules struct {
	Exact    []CompiledRule
	Wildcard []CompiledRule
}

type CompiledRule struct {
	Host      string
	GroupName string
	Rule      string
}

type RuleSetInfo struct {
	Name          string
	Version       string
	UpdatedAt     string
	GroupCount    int
	ExactCount    int
	WildcardCount int
}

type Matcher struct {
	exact    map[string]ruleEntry
	wildcard []ruleEntry
}

type ruleEntry struct {
	group string
	rule  string
	host  string
}

const (
	DefaultSteamRuleSetName      = "steam-web"
	DefaultSteamRuleSetVersion   = "2026.06.22"
	DefaultSteamRuleSetUpdatedAt = "2026-06-22"
)

var DefaultSteamRules = []RuleGroup{
	{
		Name: "store",
		Domains: []string{
			"store.steampowered.com",
			"checkout.steampowered.com",
			"help.steampowered.com",
			"login.steampowered.com",
			"media.steampowered.com",
		},
	},
	{
		Name: "community",
		Domains: []string{
			"steamcommunity.com",
			"*.steamcommunity.com",
		},
	},
	{
		Name: "api",
		Domains: []string{
			"api.steampowered.com",
			"partner.steam-api.com",
		},
	},
	{
		Name: "chat",
		Domains: []string{
			"steam-chat.com",
			"*.steam-chat.com",
		},
	},
	{
		Name: "static",
		Domains: []string{
			"community.steamstatic.com",
			"steamstatic.com",
			"*.steamstatic.com",
			"akamai.steamstatic.com",
			"*.akamai.steamstatic.com",
		},
	},
	{
		Name: "cdn",
		Domains: []string{
			"steamcdn-a.akamaihd.net",
		},
	},
}

func DefaultSteamRuleSetInfo() RuleSetInfo {
	matcher, err := NewMatcher(DefaultSteamRules, nil)
	if err != nil {
		return RuleSetInfo{
			Name:      DefaultSteamRuleSetName,
			Version:   DefaultSteamRuleSetVersion,
			UpdatedAt: DefaultSteamRuleSetUpdatedAt,
		}
	}
	compiled := matcher.Rules()
	return RuleSetInfo{
		Name:          DefaultSteamRuleSetName,
		Version:       DefaultSteamRuleSetVersion,
		UpdatedAt:     DefaultSteamRuleSetUpdatedAt,
		GroupCount:    len(DefaultSteamRules),
		ExactCount:    len(compiled.Exact),
		WildcardCount: len(compiled.Wildcard),
	}
}

func NewMatcher(groups []RuleGroup, customDomains []string) (*Matcher, error) {
	m := &Matcher{
		exact: make(map[string]ruleEntry),
	}
	for _, group := range groups {
		if err := m.addGroup(group); err != nil {
			return nil, err
		}
	}
	if len(customDomains) > 0 {
		if err := m.addGroup(RuleGroup{Name: "custom", Domains: customDomains}); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Matcher) MatchHost(host string) (MatchResult, bool) {
	normalized, err := NormalizeHost(host)
	if err != nil {
		return MatchResult{}, false
	}
	if entry, ok := m.exact[normalized]; ok {
		return MatchResult{Host: normalized, GroupName: entry.group, Rule: entry.rule}, true
	}
	for _, entry := range m.wildcard {
		if normalized != entry.host && strings.HasSuffix(normalized, "."+entry.host) {
			return MatchResult{Host: normalized, GroupName: entry.group, Rule: entry.rule}, true
		}
	}
	return MatchResult{}, false
}

func (m *Matcher) RuleCount() int {
	return len(m.exact) + len(m.wildcard)
}

func (m *Matcher) Rules() CompiledRules {
	compiled := CompiledRules{
		Exact:    make([]CompiledRule, 0, len(m.exact)),
		Wildcard: make([]CompiledRule, 0, len(m.wildcard)),
	}
	for _, entry := range m.exact {
		compiled.Exact = append(compiled.Exact, CompiledRule{
			Host:      entry.host,
			GroupName: entry.group,
			Rule:      entry.rule,
		})
	}
	for _, entry := range m.wildcard {
		compiled.Wildcard = append(compiled.Wildcard, CompiledRule{
			Host:      entry.host,
			GroupName: entry.group,
			Rule:      entry.rule,
		})
	}
	sortCompiledRules(compiled.Exact)
	sortCompiledRules(compiled.Wildcard)
	return compiled
}

func sortCompiledRules(entries []CompiledRule) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Host != entries[j].Host {
			return entries[i].Host < entries[j].Host
		}
		if entries[i].GroupName != entries[j].GroupName {
			return entries[i].GroupName < entries[j].GroupName
		}
		return entries[i].Rule < entries[j].Rule
	})
}

func (m *Matcher) addGroup(group RuleGroup) error {
	if strings.TrimSpace(group.Name) == "" {
		return fmt.Errorf("rule group name is required")
	}
	for _, domain := range group.Domains {
		entry, wildcard, err := normalizeRule(group.Name, domain)
		if err != nil {
			return err
		}
		if wildcard {
			m.wildcard = append(m.wildcard, entry)
			continue
		}
		m.exact[entry.host] = entry
	}
	return nil
}

func normalizeRule(group, rule string) (ruleEntry, bool, error) {
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return ruleEntry{}, false, fmt.Errorf("empty rule in group %q", group)
	}
	if strings.Contains(trimmed, "*") && !strings.HasPrefix(trimmed, "*.") {
		return ruleEntry{}, false, fmt.Errorf("wildcard rule %q must use the *.example.com form", rule)
	}
	if strings.HasPrefix(trimmed, "*.") {
		host, err := NormalizeHost(strings.TrimPrefix(trimmed, "*."))
		if err != nil {
			return ruleEntry{}, false, fmt.Errorf("normalize wildcard rule %q: %w", rule, err)
		}
		return ruleEntry{group: group, rule: "*." + host, host: host}, true, nil
	}
	host, err := NormalizeHost(trimmed)
	if err != nil {
		return ruleEntry{}, false, fmt.Errorf("normalize rule %q: %w", rule, err)
	}
	return ruleEntry{group: group, rule: host, host: host}, false, nil
}

func NormalizeHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return "", err
		}
		host = parsed.Host
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	host = strings.TrimSuffix(host, ".")
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.ContainsAny(host, "/?#") {
		return "", fmt.Errorf("host %q contains URL delimiters", raw)
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", err
	}
	return strings.ToLower(ascii), nil
}

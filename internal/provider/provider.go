package provider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

const (
	IDSteam  = "steam"
	IDGitHub = "github"

	StatusStable       = "stable"
	StatusExperimental = "experimental"
)

type Provider struct {
	ID               string
	Name             string
	Status           string
	RuleSetName      string
	RuleSetVersion   string
	RuleSetUpdatedAt string
	Rules            []rules.RuleGroup
	OutboundProfiles []upstream.Profile
	ProbeTargets     []upstream.ProbeTarget
}

type Summary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	RuleSetName      string `json:"rule_set_name,omitempty"`
	RuleSetVersion   string `json:"rule_set_version,omitempty"`
	RuleSetUpdatedAt string `json:"rule_set_updated_at,omitempty"`
	RuleCount        int    `json:"rule_count"`
	OutboundProfiles int    `json:"outbound_profiles"`
	ProbeTargets     int    `json:"probe_targets"`
}

func DefaultEnabled() []string {
	return []string{IDSteam}
}

func Builtins() []Provider {
	return []Provider{Steam(), GitHub()}
}

func Lookup(id string) (Provider, bool) {
	id = normalizeID(id)
	for _, p := range Builtins() {
		if p.ID == id {
			return cloneProvider(p), true
		}
	}
	return Provider{}, false
}

func ResolveEnabled(ids []string) ([]Provider, error) {
	if ids == nil {
		ids = DefaultEnabled()
	}
	seen := make(map[string]struct{})
	providers := make([]Provider, 0, len(ids))
	for _, rawID := range ids {
		id := normalizeID(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		p, ok := Lookup(id)
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", rawID)
		}
		seen[id] = struct{}{}
		providers = append(providers, p)
	}
	return providers, nil
}

func RuleGroups(providers []Provider) []rules.RuleGroup {
	var groups []rules.RuleGroup
	for _, p := range providers {
		groups = append(groups, cloneRuleGroups(p.Rules)...)
	}
	return groups
}

func OutboundProfiles(providers []Provider) []upstream.Profile {
	var profiles []upstream.Profile
	for _, p := range providers {
		profiles = append(profiles, cloneProfiles(p.OutboundProfiles)...)
	}
	return profiles
}

func ProbeTargets(providers []Provider) []upstream.ProbeTarget {
	var targets []upstream.ProbeTarget
	for _, p := range providers {
		for _, target := range p.ProbeTargets {
			target.ProviderID = p.ID
			targets = append(targets, target)
		}
	}
	return targets
}

func Summaries(providers []Provider) []Summary {
	summaries := make([]Summary, 0, len(providers))
	for _, p := range providers {
		info := rules.NewRuleSetInfo(p.RuleSetName, p.RuleSetVersion, p.RuleSetUpdatedAt, p.Rules)
		summaries = append(summaries, Summary{
			ID:               p.ID,
			Name:             p.Name,
			Status:           p.Status,
			RuleSetName:      info.Name,
			RuleSetVersion:   info.Version,
			RuleSetUpdatedAt: info.UpdatedAt,
			RuleCount:        info.ExactCount + info.WildcardCount,
			OutboundProfiles: len(p.OutboundProfiles),
			ProbeTargets:     len(p.ProbeTargets),
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})
	return summaries
}

func RuleSetInfo(p Provider) rules.RuleSetInfo {
	return rules.NewRuleSetInfo(p.RuleSetName, p.RuleSetVersion, p.RuleSetUpdatedAt, p.Rules)
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func cloneProvider(p Provider) Provider {
	p.Rules = cloneRuleGroups(p.Rules)
	p.OutboundProfiles = cloneProfiles(p.OutboundProfiles)
	p.ProbeTargets = append([]upstream.ProbeTarget(nil), p.ProbeTargets...)
	return p
}

func cloneRuleGroups(groups []rules.RuleGroup) []rules.RuleGroup {
	if len(groups) == 0 {
		return nil
	}
	cloned := make([]rules.RuleGroup, 0, len(groups))
	for _, group := range groups {
		cloned = append(cloned, rules.RuleGroup{
			Name:    group.Name,
			Domains: append([]string(nil), group.Domains...),
		})
	}
	return cloned
}

func cloneProfiles(profiles []upstream.Profile) []upstream.Profile {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]upstream.Profile, 0, len(profiles))
	for _, profile := range profiles {
		cloned = append(cloned, upstream.Profile{
			MatchDomains:          append([]string(nil), profile.MatchDomains...),
			CandidateIPs:          append([]string(nil), profile.CandidateIPs...),
			ForwardHost:           profile.ForwardHost,
			TLSServerName:         profile.TLSServerName,
			IgnoreTLSNameMismatch: profile.IgnoreTLSNameMismatch,
		})
	}
	return cloned
}

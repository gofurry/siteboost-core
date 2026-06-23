package provider

import (
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

func TestResolveEnabledDefaultsToSteam(t *testing.T) {
	providers, err := ResolveEnabled(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].ID != IDSteam {
		t.Fatalf("providers = %#v", providers)
	}
}

func TestResolveEnabledGitHubAndDedupe(t *testing.T) {
	providers, err := ResolveEnabled([]string{" github ", "steam", "github"})
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 {
		t.Fatalf("providers = %#v", providers)
	}
	if providers[0].ID != IDGitHub || providers[1].ID != IDSteam {
		t.Fatalf("providers = %#v", providers)
	}
}

func TestResolveEnabledUnknownProvider(t *testing.T) {
	_, err := ResolveEnabled([]string{"gitlab"})
	if err == nil {
		t.Fatalf("expected unknown provider error")
	}
}

func TestSteamProviderKeepsRulesProfilesAndProbes(t *testing.T) {
	steam := Steam()
	if steam.Status != StatusStable {
		t.Fatalf("steam status = %q", steam.Status)
	}
	info := RuleSetInfo(steam)
	if info.Name != SteamRuleSetName || info.Version == "" || info.UpdatedAt == "" {
		t.Fatalf("bad steam rule set info: %#v", info)
	}
	if info.GroupCount != 6 || info.ExactCount != 13 || info.WildcardCount != 4 {
		t.Fatalf("bad steam rule counts: %#v", info)
	}
	if len(steam.OutboundProfiles) != 4 {
		t.Fatalf("steam outbound profiles = %#v", steam.OutboundProfiles)
	}
	for _, host := range []string{
		"steamcdn-a.akamaihd.net",
		"community.steamstatic.com",
		"media.steampowered.com",
		"steamcommunity.com",
		"store.steampowered.com",
		"help.steampowered.com",
		"login.steampowered.com",
	} {
		t.Run(host, func(t *testing.T) {
			if !profileMatches(steam.OutboundProfiles, host) {
				t.Fatalf("steam profile does not match %s", host)
			}
		})
	}
	if len(steam.ProbeTargets) != 6 {
		t.Fatalf("steam probe targets = %#v", steam.ProbeTargets)
	}
}

func TestGitHubProviderIsExperimentalSkeleton(t *testing.T) {
	github := GitHub()
	if github.Status != StatusExperimental {
		t.Fatalf("github status = %q", github.Status)
	}
	if len(github.OutboundProfiles) != 0 {
		t.Fatalf("github should not define default outbound profiles: %#v", github.OutboundProfiles)
	}
	matcher, err := rules.NewMatcher(github.Rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"github.com", "api.github.com", "raw.githubusercontent.com"} {
		if _, ok := matcher.MatchHost(host); !ok {
			t.Fatalf("github provider does not match %s", host)
		}
	}
	if len(github.ProbeTargets) == 0 {
		t.Fatalf("github probe targets are empty")
	}
}

func profileMatches(profiles []upstream.Profile, host string) bool {
	normalized, err := rules.NormalizeHost(host)
	if err != nil {
		return false
	}
	for _, profile := range profiles {
		for _, domain := range profile.MatchDomains {
			wildcard := strings.HasPrefix(domain, "*.")
			if wildcard {
				domain = strings.TrimPrefix(domain, "*.")
			}
			normalizedDomain, err := rules.NormalizeHost(domain)
			if err != nil {
				continue
			}
			if wildcard {
				if normalized != normalizedDomain && strings.HasSuffix(normalized, "."+normalizedDomain) {
					return true
				}
				continue
			}
			if normalized == normalizedDomain {
				return true
			}
		}
	}
	return false
}

package rules

import "testing"

func TestMatcherMatchesDefaultRules(t *testing.T) {
	matcher, err := NewMatcher([]RuleGroup{
		{
			Name: "main",
			Domains: []string{
				"example.com",
				"*.example.org",
			},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		host string
		want bool
	}{
		{host: "example.com", want: true},
		{host: "EXAMPLE.COM:443", want: true},
		{host: "foo.example.org", want: true},
		{host: "example.org", want: false},
		{host: "other.test", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			_, got := matcher.MatchHost(tt.host)
			if got != tt.want {
				t.Fatalf("MatchHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestMatcherCustomDomains(t *testing.T) {
	matcher, err := NewMatcher(nil, []string{"*.例子.test", "custom.example"})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := matcher.MatchHost("a.xn--fsqu00a.test"); !ok {
		t.Fatalf("expected IDNA wildcard match")
	}
	if _, ok := matcher.MatchHost("custom.example."); !ok {
		t.Fatalf("expected custom exact match")
	}
}

func TestMatcherRejectsInvalidWildcard(t *testing.T) {
	_, err := NewMatcher([]RuleGroup{{Name: "bad", Domains: []string{"foo.*.example"}}}, nil)
	if err == nil {
		t.Fatalf("expected invalid wildcard error")
	}
}

func TestMatcherRulesExportIsDeterministic(t *testing.T) {
	matcher, err := NewMatcher([]RuleGroup{
		{Name: "b", Domains: []string{"b.example", "*.z.example"}},
		{Name: "a", Domains: []string{"a.example", "*.a.example"}},
	}, []string{"c.example"})
	if err != nil {
		t.Fatal(err)
	}

	compiled := matcher.Rules()
	gotExact := hostsOf(compiled.Exact)
	wantExact := []string{"a.example", "b.example", "c.example"}
	if !sameStrings(gotExact, wantExact) {
		t.Fatalf("exact rules = %#v, want %#v", gotExact, wantExact)
	}
	gotWildcard := hostsOf(compiled.Wildcard)
	wantWildcard := []string{"a.example", "z.example"}
	if !sameStrings(gotWildcard, wantWildcard) {
		t.Fatalf("wildcard rules = %#v, want %#v", gotWildcard, wantWildcard)
	}
}

func TestNewRuleSetInfo(t *testing.T) {
	groups := []RuleGroup{
		{Name: "a", Domains: []string{"a.example", "*.b.example"}},
	}
	info := NewRuleSetInfo("example-web", "2026.06.23", "2026-06-23", groups)
	if info.Name != "example-web" || info.Version == "" || info.UpdatedAt == "" {
		t.Fatalf("bad rule set info: %#v", info)
	}
	if info.GroupCount != len(groups) {
		t.Fatalf("group count = %d, want %d", info.GroupCount, len(groups))
	}
	if info.ExactCount == 0 || info.WildcardCount == 0 {
		t.Fatalf("rule counts were not populated: %#v", info)
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := map[string]string{
		"https://Store.SteamPowered.com/path?q=secret": "store.steampowered.com",
		"steamcommunity.com:443":                       "steamcommunity.com",
		"steamcommunity.com.":                          "steamcommunity.com",
		"例子.test":                                      "xn--fsqu00a.test",
	}

	for input, want := range tests {
		got, err := NormalizeHost(input)
		if err != nil {
			t.Fatalf("NormalizeHost(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeHost(%q) = %q, want %q", input, got, want)
		}
	}
}

func hostsOf(entries []CompiledRule) []string {
	hosts := make([]string, 0, len(entries))
	for _, entry := range entries {
		hosts = append(hosts, entry.Host)
	}
	return hosts
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

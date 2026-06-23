package provider

import (
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

const (
	GitHubRuleSetName      = "github-web"
	GitHubRuleSetVersion   = "2026.06.23"
	GitHubRuleSetUpdatedAt = "2026-06-23"
)

func GitHub() Provider {
	return Provider{
		ID:               IDGitHub,
		Name:             "GitHub",
		Status:           StatusExperimental,
		RuleSetName:      GitHubRuleSetName,
		RuleSetVersion:   GitHubRuleSetVersion,
		RuleSetUpdatedAt: GitHubRuleSetUpdatedAt,
		Rules: []rules.RuleGroup{
			{
				Name: "github/web",
				Domains: []string{
					"github.com",
					"gist.github.com",
					"*.github.com",
				},
			},
			{
				Name: "github/api",
				Domains: []string{
					"api.github.com",
				},
			},
			{
				Name: "github/raw",
				Domains: []string{
					"raw.githubusercontent.com",
					"*.githubusercontent.com",
				},
			},
			{
				Name: "github/assets",
				Domains: []string{
					"github.githubassets.com",
					"assets-cdn.github.com",
					"objects.githubusercontent.com",
					"codeload.github.com",
					"avatars.githubusercontent.com",
					"user-images.githubusercontent.com",
				},
			},
		},
		ProbeTargets: []upstream.ProbeTarget{
			{Host: "github.com", Port: "443", Path: "/"},
			{Host: "api.github.com", Port: "443", Path: "/"},
			{Host: "raw.githubusercontent.com", Port: "443", Path: "/"},
		},
	}
}

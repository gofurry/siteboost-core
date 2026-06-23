package provider

import (
	"github.com/gofurry/go-steam-core/internal/rules"
	"github.com/gofurry/go-steam-core/internal/upstream"
)

const (
	SteamRuleSetName      = "steam-web"
	SteamRuleSetVersion   = "2026.06.22"
	SteamRuleSetUpdatedAt = "2026-06-22"
)

func Steam() Provider {
	return Provider{
		ID:               IDSteam,
		Name:             "Steam",
		Status:           StatusStable,
		RuleSetName:      SteamRuleSetName,
		RuleSetVersion:   SteamRuleSetVersion,
		RuleSetUpdatedAt: SteamRuleSetUpdatedAt,
		Rules: []rules.RuleGroup{
			{
				Name: "steam/store",
				Domains: []string{
					"store.steampowered.com",
					"checkout.steampowered.com",
					"help.steampowered.com",
					"login.steampowered.com",
					"media.steampowered.com",
				},
			},
			{
				Name: "steam/community",
				Domains: []string{
					"steamcommunity.com",
					"*.steamcommunity.com",
				},
			},
			{
				Name: "steam/api",
				Domains: []string{
					"api.steampowered.com",
					"partner.steam-api.com",
				},
			},
			{
				Name: "steam/chat",
				Domains: []string{
					"steam-chat.com",
					"*.steam-chat.com",
				},
			},
			{
				Name: "steam/static",
				Domains: []string{
					"community.steamstatic.com",
					"steamstatic.com",
					"*.steamstatic.com",
					"akamai.steamstatic.com",
					"*.akamai.steamstatic.com",
				},
			},
			{
				Name: "steam/cdn",
				Domains: []string{
					"steamcdn-a.akamaihd.net",
				},
			},
		},
		OutboundProfiles: []upstream.Profile{
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
					"media.steampowered.com",
				},
				ForwardHost:   "cdn-a.akamaihd.net",
				TLSServerName: "cdn-a.akamaihd.net",
			},
			{
				MatchDomains: []string{
					"community.steamstatic.com",
				},
				ForwardHost:   "community.steamstatic.com",
				TLSServerName: "community.steamstatic.com",
			},
			{
				MatchDomains: []string{
					"steamcdn-a.akamaihd.net",
				},
				ForwardHost:   "steamcdn-a.akamaihd.net",
				TLSServerName: "steamcdn-a.akamaihd.net",
			},
		},
		ProbeTargets: []upstream.ProbeTarget{
			{Host: "steamcommunity.com", Port: "443", Path: "/"},
			{Host: "store.steampowered.com", Port: "443", Path: "/"},
			{Host: "help.steampowered.com", Port: "443", Path: "/"},
			{Host: "media.steampowered.com", Port: "443", Path: "/"},
			{Host: "community.steamstatic.com", Port: "443", Path: "/"},
			{Host: "steamcdn-a.akamaihd.net", Port: "443", Path: "/"},
		},
	}
}

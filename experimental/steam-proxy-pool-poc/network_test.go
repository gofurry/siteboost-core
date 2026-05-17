package main

import (
	"testing"
	"time"
)

func TestNormalizeProxyURL(t *testing.T) {
	candidate, err := normalizeProxyURL("127.0.0.1:7897", ProxyProtocolHTTP)
	if err != nil {
		t.Fatalf("normalizeProxyURL() error = %v", err)
	}
	if candidate.ProxyURL != "http://127.0.0.1:7897" {
		t.Fatalf("proxy url = %q", candidate.ProxyURL)
	}
	if candidate.Protocol != ProxyProtocolHTTP {
		t.Fatalf("protocol = %q", candidate.Protocol)
	}
}

func TestParseProxyServer(t *testing.T) {
	candidates := parseProxyServer("http=127.0.0.1:7890;https=127.0.0.1:7890;socks=127.0.0.1:7891")
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d", len(candidates))
	}
	if candidates[0].ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("http proxy url = %q", candidates[0].ProxyURL)
	}
	if candidates[1].ProxyURL != "https://127.0.0.1:7890" {
		t.Fatalf("https proxy url = %q", candidates[1].ProxyURL)
	}
	if candidates[2].ProxyURL != "socks5://127.0.0.1:7891" {
		t.Fatalf("socks proxy url = %q", candidates[2].ProxyURL)
	}
}

func TestCommonCandidateProtocols(t *testing.T) {
	candidates := CommonLocalCandidates()
	if len(candidates) != 8 {
		t.Fatalf("len(candidates) = %d", len(candidates))
	}
	if candidates[0].Protocol != ProxyProtocolMixed {
		t.Fatalf("first protocol = %q", candidates[0].Protocol)
	}
	if candidates[len(candidates)-1].Protocol != ProxyProtocolSOCKS5 {
		t.Fatalf("last protocol = %q", candidates[len(candidates)-1].Protocol)
	}
}

func TestSteamHTTPResponseLooksOK(t *testing.T) {
	if !steamHTTPResponseLooksOK(storeAppDetails, []byte(`{"730":{"success":true}}`)) {
		t.Fatal("expected successful appdetails JSON")
	}
	if steamHTTPResponseLooksOK(storeAppDetails, []byte(`{"730":{"success":false}}`)) {
		t.Fatal("expected failed appdetails JSON")
	}
	if steamHTTPResponseLooksOK(storeAppDetails, []byte(`REMOTE_ADDR = 127.0.0.1`)) {
		t.Fatal("expected proxy echo page to fail")
	}
}

func TestLoginRequiredIsNotNetworkFailure(t *testing.T) {
	check := checkResult("Store account", storeAccountPage, time.Now(), nil, 200, "requires_login")
	if !check.OK || check.Note != "requires_login" {
		t.Fatalf("login required check = %#v", check)
	}
}

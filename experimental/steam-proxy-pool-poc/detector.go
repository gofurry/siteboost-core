package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	storeHost        = "store.steampowered.com"
	storeAddress     = "store.steampowered.com:443"
	storeAppDetails  = "https://store.steampowered.com/api/appdetails?appids=730&filters=basic&cc=HK&l=english"
	communityHome    = "https://steamcommunity.com/"
	webAPIInfo       = "https://api.steampowered.com/ISteamWebAPIUtil/GetServerInfo/v1/"
	dynamicStoreData = "https://store.steampowered.com/dynamicstore/userdata/"
	storeAccountPage = "https://store.steampowered.com/account/"
)

type Detector struct {
	cfg ProbeConfig
}

func NewDetector(cfg ProbeConfig) *Detector {
	return &Detector{cfg: cfg}
}

func (d *Detector) RunDiagnosis(ctx context.Context, manualProxy string) (*DiagnosisReport, error) {
	direct := d.ProbeDirect(ctx)

	systemInfo := DiscoverSystemProxies()
	var system *ProbeResult
	if len(systemInfo.Candidates) > 0 {
		result := d.probeCandidate(ctx, systemInfo.Candidates[0], true)
		for _, warning := range systemInfo.Warnings {
			result.Checks = append(result.Checks, ConnectivityCheck{Name: "System proxy note", Target: "system", OK: true, Note: warning})
		}
		if systemInfo.PACURL != "" {
			result.Checks = append(result.Checks, ConnectivityCheck{Name: "PAC", Target: systemInfo.PACURL, OK: true, Note: "pac_detected_not_parsed"})
		}
		system = &result
	} else if systemInfo.PACURL != "" || len(systemInfo.Warnings) > 0 {
		result := ProbeResult{
			Candidate:  ProxyCandidate{Name: "System Proxy", Protocol: ProxyProtocolUnknown, Source: "system_proxy"},
			OK:         false,
			Error:      "no usable static system proxy",
			Suggestion: "检测到系统代理线索，但 POC 暂不解析 PAC；请改用手动代理或本机端口扫描。",
		}
		if systemInfo.PACURL != "" {
			result.Checks = append(result.Checks, ConnectivityCheck{Name: "PAC", Target: systemInfo.PACURL, OK: true, Note: "pac_detected_not_parsed"})
		}
		for _, warning := range systemInfo.Warnings {
			result.Checks = append(result.Checks, ConnectivityCheck{Name: "System proxy note", Target: "system", OK: true, Note: warning})
		}
		system = &result
	}

	local := d.ScanLocalCandidates(ctx)

	var manual *ProbeResult
	if strings.TrimSpace(manualProxy) != "" {
		result := d.TestManualProxy(ctx, manualProxy)
		manual = &result
	}

	report := &DiagnosisReport{
		Direct:          direct,
		System:          system,
		LocalCandidates: local,
		Manual:          manual,
	}
	report.Recommended = recommend(report)
	report.Summary = summarize(report)
	return report, nil
}

func (d *Detector) ProbeDirect(ctx context.Context) ProbeResult {
	start := time.Now()
	client, err := buildHTTPClient(nil, d.cfg.HTTPTimeout)
	result := ProbeResult{
		Candidate: ProxyCandidate{Name: "Direct", Protocol: ProxyProtocolUnknown, Source: string(NetworkModeDirect)},
	}
	if err != nil {
		result.Error = err.Error()
		result.Suggestion = suggestionForError(err)
		return result
	}

	checks := []ConnectivityCheck{
		checkDNS(ctx, storeHost),
		checkTCP(ctx, storeAddress, d.cfg.DialTimeout),
		checkTLS(ctx, storeAddress, d.cfg.DialTimeout),
		checkHTTP(ctx, client, "Store appdetails", storeAppDetails, false),
		checkHTTP(ctx, client, "Community home", communityHome, false),
		checkHTTP(ctx, client, "Steam Web API", webAPIInfo, false),
		checkHTTP(ctx, client, "Dynamic store userdata", dynamicStoreData, true),
		checkHTTP(ctx, client, "Store account", storeAccountPage, true),
	}
	result.Checks = checks
	result.DurationMS = time.Since(start).Milliseconds()
	result.OK = requiredChecksOK(checks)
	result.Error = firstError(checks)
	result.Suggestion = suggestionForProbe(result)
	return result
}

func (d *Detector) ScanLocalCandidates(ctx context.Context) []ProbeResult {
	candidates := CommonLocalCandidates()
	results := make([]ProbeResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, d.probeCandidate(ctx, candidate, true))
	}
	return results
}

func (d *Detector) TestManualProxy(ctx context.Context, manualProxy string) ProbeResult {
	candidate, err := normalizeProxyURL(manualProxy, ProxyProtocolHTTP)
	if err != nil {
		return ProbeResult{
			Candidate:  ProxyCandidate{Name: "Manual Proxy", Address: manualProxy, Source: "manual", Protocol: ProxyProtocolUnknown},
			OK:         false,
			Error:      err.Error(),
			Suggestion: "请输入类似 127.0.0.1:7897、http://127.0.0.1:7897 或 socks5://127.0.0.1:1080 的代理地址。",
		}
	}
	candidate.Name = "Manual Proxy"
	candidate.Source = "manual"
	return d.probeCandidate(ctx, candidate, true)
}

func (d *Detector) probeCandidate(ctx context.Context, candidate ProxyCandidate, tryMixed bool) ProbeResult {
	candidate = normalizeCandidate(candidate)
	start := time.Now()
	result := ProbeResult{Candidate: candidate}

	if candidate.Address != "" {
		portCheck := checkPortOpen(ctx, candidate.Address, d.cfg.DialTimeout)
		result.Checks = append(result.Checks, portCheck)
		if !portCheck.OK {
			result.DurationMS = time.Since(start).Milliseconds()
			result.OK = false
			result.Error = portCheck.Error
			result.Suggestion = "端口不可连接，请确认代理客户端已启动且端口正确。"
			return result
		}
	}

	protocols := protocolsToTry(candidate.Protocol, tryMixed)
	var last ProbeResult
	for _, protocol := range protocols {
		testCandidate := candidate
		testCandidate.Protocol = protocol
		testCandidate.ProxyURL = proxyURLFor(testCandidate.Address, protocol)
		probed := d.probeProxyHTTP(ctx, testCandidate, start)
		if probed.OK {
			probed.Checks = append([]ConnectivityCheck{result.Checks[0]}, probed.Checks...)
			probed.Candidate.Protocol = protocol
			if candidate.Protocol == ProxyProtocolMixed {
				probed.Candidate.Protocol = ProxyProtocolMixed
			}
			return probed
		}
		last = probed
		result.Checks = append(result.Checks, probed.Checks...)
	}

	if len(protocols) == 0 {
		result.Error = "no proxy protocol to test"
	} else if last.Error != "" {
		result.Error = last.Error
	}
	result.DurationMS = time.Since(start).Milliseconds()
	result.OK = false
	result.Suggestion = suggestionForProbe(result)
	return result
}

func (d *Detector) probeProxyHTTP(ctx context.Context, candidate ProxyCandidate, start time.Time) ProbeResult {
	client, err := buildHTTPClient(&candidate, d.cfg.HTTPTimeout)
	result := ProbeResult{Candidate: candidate}
	if err != nil {
		result.Error = err.Error()
		result.DurationMS = time.Since(start).Milliseconds()
		result.Suggestion = suggestionForError(err)
		return result
	}
	checks := []ConnectivityCheck{
		checkHTTP(ctx, client, "Store appdetails", storeAppDetails, false),
		checkHTTP(ctx, client, "Community home", communityHome, false),
		checkHTTP(ctx, client, "Steam Web API", webAPIInfo, false),
		checkHTTP(ctx, client, "Dynamic store userdata", dynamicStoreData, true),
		checkHTTP(ctx, client, "Store account", storeAccountPage, true),
	}
	result.Checks = checks
	result.DurationMS = time.Since(start).Milliseconds()
	result.OK = requiredChecksOK(checks)
	result.Error = firstError(checks)
	result.Suggestion = suggestionForProbe(result)
	return result
}

func protocolsToTry(protocol ProxyProtocol, tryMixed bool) []ProxyProtocol {
	switch protocol {
	case ProxyProtocolHTTP, ProxyProtocolHTTPS, ProxyProtocolSOCKS5:
		return []ProxyProtocol{protocol}
	case ProxyProtocolMixed:
		if tryMixed {
			return []ProxyProtocol{ProxyProtocolHTTP, ProxyProtocolSOCKS5}
		}
		return []ProxyProtocol{ProxyProtocolHTTP}
	default:
		return []ProxyProtocol{ProxyProtocolHTTP, ProxyProtocolSOCKS5}
	}
}

func proxyURLFor(address string, protocol ProxyProtocol) string {
	switch protocol {
	case ProxyProtocolSOCKS5:
		return "socks5://" + address
	case ProxyProtocolHTTPS:
		return "https://" + address
	default:
		return "http://" + address
	}
}

func requiredChecksOK(checks []ConnectivityCheck) bool {
	required := map[string]bool{
		"Store appdetails": false,
		"Community home":   false,
		"Steam Web API":    false,
	}
	for _, check := range checks {
		if _, ok := required[check.Name]; ok && check.OK {
			required[check.Name] = true
		}
	}
	for _, ok := range required {
		if !ok {
			return false
		}
	}
	return true
}

func firstError(checks []ConnectivityCheck) string {
	for _, check := range checks {
		if !check.OK && check.Error != "" {
			return fmt.Sprintf("%s: %s", check.Name, check.Error)
		}
	}
	return ""
}

func recommend(report *DiagnosisReport) *ProbeResult {
	if report.Manual != nil && report.Manual.OK {
		return report.Manual
	}
	if report.Direct.OK {
		return &report.Direct
	}
	if report.System != nil && report.System.OK {
		return report.System
	}
	for i := range report.LocalCandidates {
		if report.LocalCandidates[i].OK {
			return &report.LocalCandidates[i]
		}
	}
	return nil
}

func summarize(report *DiagnosisReport) string {
	if report.Recommended == nil {
		return "未找到可用的 Steam 请求出口。请确认网络连接、代理客户端和端口配置。"
	}
	switch report.Recommended.Candidate.Source {
	case string(NetworkModeDirect):
		return "直连 Steam 可用，当前无需代理。"
	case "system_proxy":
		return "系统代理可用，建议使用系统代理作为 SteamScope 请求出口。"
	case "common_port":
		return "检测到可用本机代理端口，建议使用该端口作为 SteamScope 请求出口。"
	case "manual":
		return "手动代理可用，可作为 SteamScope 请求出口。"
	default:
		return "已找到可用请求出口。"
	}
}

func suggestionForProbe(result ProbeResult) string {
	if result.OK {
		return "该出口可用于 SteamScope 请求。"
	}
	if result.Error == "" {
		return "该出口未通过完整 Steam 检测。"
	}
	return suggestionForError(errors.New(result.Error))
}

func suggestionForError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "timeout") || strings.Contains(text, "deadline"):
		return "请求超时，可能是 Steam 无法直连或该代理节点不可用。"
	case strings.Contains(text, "connection refused"):
		return "连接被拒绝，请确认代理客户端已启动且端口正确。"
	case strings.Contains(text, "socks"):
		return "SOCKS5 握手失败，请确认该端口是否支持 SOCKS5。"
	case strings.Contains(text, "certificate") || strings.Contains(text, "tls"):
		return "TLS 或证书校验失败，该代理可能拦截了 HTTPS 流量，不建议承载登录态。"
	default:
		return "请尝试切换直连、系统代理或本机代理端口。"
	}
}

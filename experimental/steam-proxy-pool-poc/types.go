package main

type NetworkMode string

const (
	NetworkModeDirect NetworkMode = "direct"
	NetworkModeSystem NetworkMode = "system"
	NetworkModeCustom NetworkMode = "custom"
	NetworkModeAuto   NetworkMode = "auto"
)

type ProxyProtocol string

const (
	ProxyProtocolHTTP    ProxyProtocol = "http"
	ProxyProtocolHTTPS   ProxyProtocol = "https"
	ProxyProtocolSOCKS5  ProxyProtocol = "socks5"
	ProxyProtocolMixed   ProxyProtocol = "mixed"
	ProxyProtocolUnknown ProxyProtocol = "unknown"
)

type ProxyCandidate struct {
	Name     string        `json:"name"`
	Address  string        `json:"address"`
	ProxyURL string        `json:"proxyUrl"`
	Protocol ProxyProtocol `json:"protocol"`
	Source   string        `json:"source"`
}

type ConnectivityCheck struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	Note       string `json:"note,omitempty"`
}

type ProbeResult struct {
	Candidate  ProxyCandidate      `json:"candidate"`
	OK         bool                `json:"ok"`
	Checks     []ConnectivityCheck `json:"checks"`
	DurationMS int64               `json:"durationMs"`
	Error      string              `json:"error,omitempty"`
	Suggestion string              `json:"suggestion,omitempty"`
}

type DiagnosisReport struct {
	Direct          ProbeResult   `json:"direct"`
	System          *ProbeResult  `json:"system,omitempty"`
	LocalCandidates []ProbeResult `json:"localCandidates"`
	Manual          *ProbeResult  `json:"manual,omitempty"`
	Recommended     *ProbeResult  `json:"recommended,omitempty"`
	Summary         string        `json:"summary"`
}

type SystemProxyInfo struct {
	Candidates []ProxyCandidate `json:"candidates"`
	PACURL     string           `json:"pacUrl,omitempty"`
	Warnings   []string         `json:"warnings,omitempty"`
}

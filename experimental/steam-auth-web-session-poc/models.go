package main

type SteamAccountSummary struct {
	SteamID     string `json:"steamId"`
	Account     string `json:"account"`
	LoggedIn    bool   `json:"loggedIn"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
}

type QRLoginStartResult struct {
	LoginID           string        `json:"loginId"`
	QRChallengeURL    string        `json:"qrChallengeUrl"`
	Status            string        `json:"status"`
	PollIntervalSecs  int           `json:"pollIntervalSecs"`
	ValidActions      []GuardAction `json:"validActions,omitempty"`
	ExpiresAt         string        `json:"expiresAt"`
	SafeStatusMessage string        `json:"safeStatusMessage"`
}

type CredentialLoginStartRequest struct {
	AccountName string `json:"accountName"`
	Password    string `json:"password"`
}

type LoginStartResult struct {
	LoginID           string        `json:"loginId"`
	Status            string        `json:"status"`
	PollIntervalSecs  int           `json:"pollIntervalSecs"`
	ValidActions      []GuardAction `json:"validActions,omitempty"`
	ExpiresAt         string        `json:"expiresAt"`
	SafeStatusMessage string        `json:"safeStatusMessage"`
}

type GuardAction struct {
	Type   string `json:"type"`
	Detail string `json:"detail,omitempty"`
}

type SubmitGuardCodeRequest struct {
	LoginID string `json:"loginId"`
	Code    string `json:"code"`
	Type    string `json:"type"`
}

type LoginStatus struct {
	LoginID           string        `json:"loginId"`
	Status            string        `json:"status"`
	SteamID           string        `json:"steamId,omitempty"`
	Account           string        `json:"account,omitempty"`
	PollIntervalSecs  int           `json:"pollIntervalSecs"`
	ValidActions      []GuardAction `json:"validActions,omitempty"`
	ExpiresAt         string        `json:"expiresAt,omitempty"`
	SafeStatusMessage string        `json:"safeStatusMessage"`
}

type WebSessionTestResult struct {
	OK                  bool   `json:"ok"`
	SteamID             string `json:"steamId,omitempty"`
	Account             string `json:"account,omitempty"`
	CookieDomains       int    `json:"cookieDomains"`
	CommunityOK         bool   `json:"communityOk"`
	StoreOK             bool   `json:"storeOk"`
	LastCookieRefreshAt string `json:"lastCookieRefreshAt,omitempty"`
	Message             string `json:"message"`
}

type NetworkConfigRequest struct {
	ProxyURL string `json:"proxyUrl"`
}

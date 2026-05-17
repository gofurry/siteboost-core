package steamauth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const (
	GuardUnknown            uint64 = 0
	GuardNone               uint64 = 1
	GuardEmailCode          uint64 = 2
	GuardDeviceCode         uint64 = 3
	GuardDeviceConfirmation uint64 = 4
	GuardEmailConfirmation  uint64 = 5
	GuardMachineToken       uint64 = 6

	PlatformWebBrowser uint64 = 2
	SessionPersistent  uint64 = 1

	SteamLanguageSchinese uint64 = 6
)

const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type AllowedConfirmation struct {
	Type    uint64
	Message string
}

type QRStartResponse struct {
	ClientID             uint64
	ChallengeURL         string
	RequestID            []byte
	Interval             float32
	AllowedConfirmations []AllowedConfirmation
	Version              int32
}

type CredentialStartResponse struct {
	ClientID             uint64
	RequestID            []byte
	Interval             float32
	AllowedConfirmations []AllowedConfirmation
	SteamID              string
	WeakToken            string
}

type SubmitGuardCodeRequest struct {
	ClientID uint64
	SteamID  string
	Code     string
	CodeType uint64
}

type PollResponse struct {
	NewClientID          uint64
	NewChallengeURL      string
	RefreshToken         string
	AccessToken          string
	HadRemoteInteraction bool
	AccountName          string
	NewGuardData         string
}

func (p PollResponse) SteamID() string {
	if p.RefreshToken != "" {
		return SteamIDFromJWT(p.RefreshToken)
	}
	if p.AccessToken != "" {
		return SteamIDFromJWT(p.AccessToken)
	}
	return ""
}

func GuardTypeName(value uint64) string {
	switch value {
	case GuardNone:
		return "none"
	case GuardEmailCode:
		return "email_code"
	case GuardDeviceCode:
		return "device_code"
	case GuardDeviceConfirmation:
		return "device_confirmation"
	case GuardEmailConfirmation:
		return "email_confirmation"
	case GuardMachineToken:
		return "machine_token"
	default:
		return "unknown"
	}
}

func GuardTypeFromString(value string) (uint64, error) {
	switch strings.TrimSpace(value) {
	case "email_code":
		return GuardEmailCode, nil
	case "device_code":
		return GuardDeviceCode, nil
	default:
		return 0, errors.New("unsupported guard code type")
	}
}

func SteamIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Sub
}

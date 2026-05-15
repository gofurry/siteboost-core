package steamauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestDecodeQRStartResponse(t *testing.T) {
	var confirmation []byte
	confirmation = protowire.AppendTag(confirmation, 1, protowire.VarintType)
	confirmation = protowire.AppendVarint(confirmation, GuardDeviceConfirmation)
	confirmation = protowire.AppendTag(confirmation, 2, protowire.BytesType)
	confirmation = protowire.AppendString(confirmation, "mobile")

	var payload []byte
	payload = protowire.AppendTag(payload, 1, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 123)
	payload = protowire.AppendTag(payload, 2, protowire.BytesType)
	payload = protowire.AppendString(payload, "https://example.test/qr")
	payload = protowire.AppendTag(payload, 3, protowire.BytesType)
	payload = protowire.AppendBytes(payload, []byte{1, 2, 3})
	payload = protowire.AppendTag(payload, 5, protowire.BytesType)
	payload = protowire.AppendBytes(payload, confirmation)

	got, err := decodeQRStartResponse(payload)
	if err != nil {
		t.Fatalf("decodeQRStartResponse() error = %v", err)
	}
	if got.ClientID != 123 || got.ChallengeURL == "" || len(got.RequestID) != 3 {
		t.Fatalf("unexpected qr response: %#v", got)
	}
	if len(got.AllowedConfirmations) != 1 || got.AllowedConfirmations[0].Type != GuardDeviceConfirmation {
		t.Fatalf("unexpected confirmations: %#v", got.AllowedConfirmations)
	}
}

func TestSteamIDFromJWT(t *testing.T) {
	header := map[string]string{"alg": "none"}
	payload := map[string]string{"sub": "76561198000000000"}
	token := encodeJWTPart(header) + "." + encodeJWTPart(payload) + "."

	if got := SteamIDFromJWT(token); got != "76561198000000000" {
		t.Fatalf("SteamIDFromJWT() = %q", got)
	}
}

func TestEncodeBeginCredentialsRequestIncludesTopLevelPlatform(t *testing.T) {
	payload, err := encodeBeginCredentialsRequest("account", "encrypted", "123", DefaultUserAgent)
	if err != nil {
		t.Fatalf("encodeBeginCredentialsRequest() error = %v", err)
	}

	fields := map[protowire.Number]uint64{}
	strings := map[protowire.Number]string{}
	err = consumeFields(payload, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch typ {
		case protowire.VarintType:
			fields[num] = mustVarint(value)
		case protowire.BytesType:
			strings[num] = string(value)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("consumeFields() error = %v", err)
	}
	if strings[1] == "" {
		t.Fatal("device_friendly_name field was not encoded")
	}
	if fields[6] != PlatformWebBrowser {
		t.Fatalf("platform_type = %d, want %d", fields[6], PlatformWebBrowser)
	}
	if strings[8] != "Store" {
		t.Fatalf("website_id = %q, want Store", strings[8])
	}
	if fields[11] != SteamLanguageSchinese {
		t.Fatalf("language = %d, want %d", fields[11], SteamLanguageSchinese)
	}
}

func encodeJWTPart(value any) string {
	content, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(content)
}

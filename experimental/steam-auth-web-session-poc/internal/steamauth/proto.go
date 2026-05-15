package steamauth

import (
	"encoding/binary"
	"errors"
	"math"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

func encodeGetRSAKeyRequest(accountName string) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, accountName)
	return b
}

func encodeDeviceDetails(userAgent string) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, userAgent)
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, PlatformWebBrowser)
	return b
}

func encodeBeginQRRequest(userAgent string) []byte {
	device := encodeDeviceDetails(userAgent)
	var b []byte
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendBytes(b, device)
	return b
}

func encodeBeginCredentialsRequest(accountName, encryptedPassword, timestamp, userAgent string) ([]byte, error) {
	ts, err := strconv.ParseUint(timestamp, 10, 64)
	if err != nil {
		return nil, err
	}

	device := encodeDeviceDetails(userAgent)
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, userAgent)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, accountName)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendString(b, encryptedPassword)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, ts)
	b = protowire.AppendTag(b, 5, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	b = protowire.AppendTag(b, 6, protowire.VarintType)
	b = protowire.AppendVarint(b, PlatformWebBrowser)
	b = protowire.AppendTag(b, 7, protowire.VarintType)
	b = protowire.AppendVarint(b, SessionPersistent)
	b = protowire.AppendTag(b, 8, protowire.BytesType)
	b = protowire.AppendString(b, "Store")
	b = protowire.AppendTag(b, 9, protowire.BytesType)
	b = protowire.AppendBytes(b, device)
	b = protowire.AppendTag(b, 11, protowire.VarintType)
	b = protowire.AppendVarint(b, SteamLanguageSchinese)
	return b, nil
}

func encodePollRequest(clientID uint64, requestID []byte) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, clientID)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendBytes(b, requestID)
	return b
}

func encodeSubmitGuardCodeRequest(req SubmitGuardCodeRequest) ([]byte, error) {
	steamID, err := strconv.ParseUint(req.SteamID, 10, 64)
	if err != nil {
		return nil, err
	}
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, req.ClientID)
	b = protowire.AppendTag(b, 2, protowire.Fixed64Type)
	b = protowire.AppendFixed64(b, steamID)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendString(b, req.Code)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, req.CodeType)
	return b, nil
}

type rsaKeyResponse struct {
	Mod       string
	Exp       string
	Timestamp string
}

func decodeRSAKeyResponse(data []byte) (rsaKeyResponse, error) {
	var out rsaKeyResponse
	err := consumeFields(data, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch num {
		case 1:
			out.Mod = string(value)
		case 2:
			out.Exp = string(value)
		case 3:
			out.Timestamp = strconv.FormatUint(mustVarint(value), 10)
		}
		return nil
	})
	return out, err
}

func decodeQRStartResponse(data []byte) (QRStartResponse, error) {
	var out QRStartResponse
	err := consumeFields(data, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch num {
		case 1:
			out.ClientID = mustVarint(value)
		case 2:
			out.ChallengeURL = string(value)
		case 3:
			out.RequestID = append([]byte(nil), value...)
		case 4:
			out.Interval = math.Float32frombits(uint32(binary.LittleEndian.Uint32(value)))
		case 5:
			item, err := decodeAllowedConfirmation(value)
			if err != nil {
				return err
			}
			out.AllowedConfirmations = append(out.AllowedConfirmations, item)
		case 6:
			out.Version = int32(mustVarint(value))
		}
		return nil
	})
	return out, err
}

func decodeCredentialStartResponse(data []byte) (CredentialStartResponse, error) {
	var out CredentialStartResponse
	err := consumeFields(data, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch num {
		case 1:
			out.ClientID = mustVarint(value)
		case 2:
			out.RequestID = append([]byte(nil), value...)
		case 3:
			out.Interval = math.Float32frombits(uint32(binary.LittleEndian.Uint32(value)))
		case 4:
			item, err := decodeAllowedConfirmation(value)
			if err != nil {
				return err
			}
			out.AllowedConfirmations = append(out.AllowedConfirmations, item)
		case 5:
			out.SteamID = strconv.FormatUint(mustVarint(value), 10)
		case 6:
			out.WeakToken = string(value)
		}
		return nil
	})
	return out, err
}

func decodePollResponse(data []byte) (PollResponse, error) {
	var out PollResponse
	err := consumeFields(data, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch num {
		case 1:
			out.NewClientID = mustVarint(value)
		case 2:
			out.NewChallengeURL = string(value)
		case 3:
			out.RefreshToken = string(value)
		case 4:
			out.AccessToken = string(value)
		case 5:
			out.HadRemoteInteraction = mustVarint(value) != 0
		case 6:
			out.AccountName = string(value)
		case 7:
			out.NewGuardData = string(value)
		}
		return nil
	})
	return out, err
}

func decodeAllowedConfirmation(data []byte) (AllowedConfirmation, error) {
	var out AllowedConfirmation
	err := consumeFields(data, func(num protowire.Number, typ protowire.Type, value []byte) error {
		switch num {
		case 1:
			out.Type = mustVarint(value)
		case 2:
			out.Message = string(value)
		}
		return nil
	})
	return out, err
}

func mustVarint(data []byte) uint64 {
	value, _ := protowire.ConsumeVarint(data)
	return value
}

func consumeFields(data []byte, fn func(protowire.Number, protowire.Type, []byte) error) error {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return errors.New("invalid protobuf tag")
		}
		data = data[n:]

		var raw []byte
		switch typ {
		case protowire.VarintType:
			_, n = protowire.ConsumeVarint(data)
			if n < 0 {
				return errors.New("invalid protobuf varint")
			}
			raw = data[:n]
		case protowire.Fixed64Type:
			_, n = protowire.ConsumeFixed64(data)
			if n < 0 {
				return errors.New("invalid protobuf fixed64")
			}
			raw = data[:n]
		case protowire.BytesType:
			var value []byte
			value, n = protowire.ConsumeBytes(data)
			if n < 0 {
				return errors.New("invalid protobuf bytes")
			}
			raw = value
		case protowire.Fixed32Type:
			_, n = protowire.ConsumeFixed32(data)
			if n < 0 {
				return errors.New("invalid protobuf fixed32")
			}
			raw = data[:n]
		default:
			return errors.New("unsupported protobuf wire type")
		}

		if err := fn(num, typ, raw); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

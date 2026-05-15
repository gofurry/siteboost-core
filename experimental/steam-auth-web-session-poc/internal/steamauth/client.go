package steamauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const webAPIBase = "https://api.steampowered.com"

type Client struct {
	http      *http.Client
	userAgent string
}

func NewClient(client *http.Client, userAgent string) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	return &Client{http: client, userAgent: userAgent}
}

func (c *Client) BeginQR(ctx context.Context) (QRStartResponse, error) {
	body := encodeBeginQRRequest(c.userAgent)
	resp, err := c.sendProto(ctx, "Authentication", "BeginAuthSessionViaQR", 1, http.MethodPost, body)
	if err != nil {
		return QRStartResponse{}, err
	}
	return decodeQRStartResponse(resp)
}

func (c *Client) BeginCredentials(ctx context.Context, accountName, password string) (CredentialStartResponse, error) {
	keyBytes, err := c.sendProto(ctx, "Authentication", "GetPasswordRSAPublicKey", 1, http.MethodGet, encodeGetRSAKeyRequest(accountName))
	if err != nil {
		return CredentialStartResponse{}, err
	}
	key, err := decodeRSAKeyResponse(keyBytes)
	if err != nil {
		return CredentialStartResponse{}, err
	}

	encrypted, err := encryptPassword(password, key)
	if err != nil {
		return CredentialStartResponse{}, err
	}
	req, err := encodeBeginCredentialsRequest(accountName, encrypted, key.Timestamp, c.userAgent)
	if err != nil {
		return CredentialStartResponse{}, err
	}

	resp, err := c.sendProto(ctx, "Authentication", "BeginAuthSessionViaCredentials", 1, http.MethodPost, req)
	if err != nil {
		return CredentialStartResponse{}, err
	}
	return decodeCredentialStartResponse(resp)
}

func (c *Client) SubmitGuardCode(ctx context.Context, req SubmitGuardCodeRequest) error {
	body, err := encodeSubmitGuardCodeRequest(req)
	if err != nil {
		return err
	}
	_, err = c.sendProto(ctx, "Authentication", "UpdateAuthSessionWithSteamGuardCode", 1, http.MethodPost, body)
	return err
}

func (c *Client) Poll(ctx context.Context, clientID uint64, requestID []byte) (PollResponse, error) {
	body := encodePollRequest(clientID, requestID)
	resp, err := c.sendProto(ctx, "Authentication", "PollAuthSessionStatus", 1, http.MethodPost, body)
	if err != nil {
		return PollResponse{}, err
	}
	return decodePollResponse(resp)
}

func (c *Client) sendProto(ctx context.Context, apiInterface, method string, version int, httpMethod string, payload []byte) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/I%sService/%s/v%d/", webAPIBase, apiInterface, method, version)

	var body io.Reader
	reqURL := endpoint
	headers := c.apiHeaders()
	encoded := base64.StdEncoding.EncodeToString(payload)

	if httpMethod == http.MethodGet {
		values := url.Values{}
		if encoded != "" {
			values.Set("input_protobuf_encoded", encoded)
		}
		reqURL += "?" + values.Encode()
	} else {
		buf := &bytes.Buffer{}
		writer := multipart.NewWriter(buf)
		if encoded != "" {
			if err := writer.WriteField("input_protobuf_encoded", encoded); err != nil {
				return nil, err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		body = buf
		headers.Set("Content-Type", writer.FormDataContentType())
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steam auth HTTP %d", resp.StatusCode)
	}
	if eresult := resp.Header.Get("x-eresult"); eresult != "" && eresult != "1" {
		message := resp.Header.Get("x-error_message")
		if message == "" {
			message = "steam auth failed with EResult " + eresult
			if detail := eresultMessage(eresult); detail != "" {
				message += " (" + detail + ")"
			}
		}
		return nil, errors.New(message)
	}
	if len(content) == 0 {
		return nil, nil
	}

	var jsonProbe map[string]json.RawMessage
	if json.Unmarshal(content, &jsonProbe) == nil {
		if raw, ok := jsonProbe["response"]; ok {
			var responseText string
			if json.Unmarshal(raw, &responseText) == nil {
				return base64.StdEncoding.DecodeString(responseText)
			}
		}
	}

	return content, nil
}

func eresultMessage(value string) string {
	switch value {
	case "29":
		return "DuplicateRequest: Steam detected a duplicate pending login request; wait a few minutes before starting a new login"
	case "63":
		return "AccountLogonDenied: Steam Guard confirmation is required"
	case "65":
		return "InvalidLoginAuthCode"
	case "71":
		return "ExpiredLoginAuthCode"
	case "84":
		return "RateLimitExceeded"
	default:
		return ""
	}
}

func (c *Client) apiHeaders() http.Header {
	headers := http.Header{}
	headers.Set("Accept", "application/x-protobuf, application/json, text/plain, */*")
	headers.Set("Origin", "https://store.steampowered.com")
	headers.Set("Referer", "https://store.steampowered.com/login/")
	headers.Set("User-Agent", c.userAgent)
	return headers
}

func formRequest(ctx context.Context, method, target string, fields map[string]string, headers http.Header) (*http.Request, error) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, buf)
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func formURLEncodedRequest(ctx context.Context, method, target string, fields map[string]string, headers http.Header) (*http.Request, error) {
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	return req, nil
}

func parseUint(value string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return n
}

func httpClientWithTimeout(base *http.Client, timeout time.Duration) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	next := *base
	next.Timeout = timeout
	return &next
}

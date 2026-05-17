package steamauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type finalizeLoginResponse struct {
	Error        int            `json:"error"`
	TransferInfo []transferInfo `json:"transfer_info"`
}

type transferInfo struct {
	URL    string            `json:"url"`
	Params map[string]string `json:"params"`
}

func (c *Client) GetWebCookieJar(ctx context.Context, refreshToken string) (*cookiejar.Jar, int, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, 0, errors.New("refresh token is required")
	}

	sessionID := randomSessionID()
	headers := c.apiHeaders()
	headers.Set("Origin", "https://steamcommunity.com")
	headers.Set("Referer", "https://steamcommunity.com/")

	req, err := formRequest(ctx, http.MethodPost, "https://login.steampowered.com/jwt/finalizelogin", map[string]string{
		"nonce":     refreshToken,
		"sessionid": sessionID,
		"redir":     "https://steamcommunity.com/login/home/?goto=",
	}, headers)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("finalizelogin HTTP %d", resp.StatusCode)
	}

	var payload finalizeLoginResponse
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, 0, err
	}
	if payload.Error != 0 {
		return nil, 0, fmt.Errorf("finalizelogin returned EResult %d", payload.Error)
	}
	if len(payload.TransferInfo) == 0 {
		return nil, 0, errors.New("finalizelogin response did not include transfer_info")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, 0, err
	}
	domains := map[string]bool{}
	setResponseCookies(jar, resp, domains)

	steamID := SteamIDFromJWT(refreshToken)
	for _, transfer := range payload.TransferInfo {
		fields := map[string]string{"steamID": steamID}
		for key, value := range transfer.Params {
			fields[key] = value
		}
		if err := c.executeTransfer(ctx, jar, transfer.URL, fields, domains); err != nil {
			return nil, 0, err
		}
	}

	for _, domain := range []string{"steamcommunity.com", "store.steampowered.com", "help.steampowered.com"} {
		target, _ := url.Parse("https://" + domain + "/")
		jar.SetCookies(target, []*http.Cookie{{
			Name:     "sessionid",
			Value:    sessionID,
			Path:     "/",
			Secure:   true,
			HttpOnly: false,
		}, {
			Name:     "Steam_Language",
			Value:    "english",
			Path:     "/",
			Secure:   true,
			HttpOnly: false,
		}})
		domains[domain] = true
	}

	return jar, len(domains), nil
}

func (c *Client) executeTransfer(ctx context.Context, jar *cookiejar.Jar, target string, fields map[string]string, domains map[string]bool) error {
	client := *c.http
	client.Jar = jar

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := formURLEncodedRequest(ctx, http.MethodPost, target, fields, c.apiHeaders())
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("transfer HTTP %d", resp.StatusCode)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if len(resp.Cookies()) == 0 {
			lastErr = errors.New("transfer returned no cookies")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		setResponseCookies(jar, resp, domains)
		return nil
	}
	return lastErr
}

func (c *Client) TestCommunitySession(ctx context.Context, jar *cookiejar.Jar, steamID string) error {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return errors.New("steam id is required to verify community session")
	}
	if !jarHasCookie(jar, "steamcommunity.com", "steamLoginSecure") {
		return errors.New("community cookie jar does not contain steamLoginSecure")
	}

	client := *c.http
	client.Jar = jar
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://steamcommunity.com/profiles/"+steamID+"/?xml=1", nil)
	if err != nil {
		return err
	}
	req.Header = c.apiHeaders()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("community session HTTP %d", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(resp.Request.URL.String()), "/login") {
		return errors.New("community session redirected to login")
	}
	body := strings.ToLower(string(content))
	if !strings.Contains(body, "<steamid64>"+steamID+"</steamid64>") {
		return errors.New("community session returned a different profile")
	}
	return nil
}

func (c *Client) TestStoreSession(ctx context.Context, jar *cookiejar.Jar, steamID string) error {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return errors.New("steam id is required to verify store session")
	}
	if !jarHasCookie(jar, "store.steampowered.com", "steamLoginSecure") {
		return errors.New("store cookie jar does not contain steamLoginSecure")
	}
	if cookieValue(jar, "store.steampowered.com", "sessionid") == "" {
		return errors.New("store cookie jar does not contain sessionid")
	}

	client := *c.http
	client.Jar = jar
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://store.steampowered.com/account/?l=english", nil)
	if err != nil {
		return err
	}
	req.Header = c.apiHeaders()
	req.Header.Set("Origin", "https://store.steampowered.com")
	req.Header.Set("Referer", "https://store.steampowered.com/")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("store session HTTP %d", resp.StatusCode)
	}
	finalURL := strings.ToLower(resp.Request.URL.String())
	if strings.Contains(finalURL, "/login") {
		return errors.New("store session redirected to login")
	}
	body := strings.ToLower(string(content))
	if !strings.Contains(body, strings.ToLower(steamID)) && !strings.Contains(body, "account_name") && !strings.Contains(body, "account details") {
		return errors.New("store session did not look authenticated")
	}
	return nil
}

type FreeLicenseResult struct {
	AlreadyOwned bool
}

type freeLicenseAttempt struct {
	Endpoint string
	Ajax     bool
}

func (c *Client) AddFreeLicense(ctx context.Context, jar *cookiejar.Jar, appID string, packageID int64) (FreeLicenseResult, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return FreeLicenseResult{}, errors.New("app id is required")
	}
	if packageID <= 0 {
		return FreeLicenseResult{}, errors.New("package id is required")
	}
	sessionID := cookieValue(jar, "store.steampowered.com", "sessionid")
	if sessionID == "" {
		return FreeLicenseResult{}, errors.New("store cookie jar does not contain sessionid")
	}
	if !jarHasCookie(jar, "store.steampowered.com", "steamLoginSecure") {
		return FreeLicenseResult{}, errors.New("store cookie jar does not contain steamLoginSecure")
	}

	client := *c.http
	client.Jar = jar

	form, endpoint, err := c.freeLicenseForm(ctx, &client, appID, packageID)
	if err != nil {
		return FreeLicenseResult{}, err
	}
	if form.Get("sessionid") == "" {
		form.Set("sessionid", sessionID)
	}
	if form.Get("action") == "" {
		form.Set("action", "add_to_cart")
	}
	if form.Get("subid") == "" {
		form.Set("subid", fmt.Sprintf("%d", packageID))
	}

	attempts := []freeLicenseAttempt{
		{Endpoint: "https://store.steampowered.com/checkout/addfreelicense", Ajax: true},
		{Endpoint: "https://store.steampowered.com/checkout/addfreelicense/", Ajax: true},
		{Endpoint: endpoint, Ajax: false},
	}

	var lastErr error
	for _, attempt := range dedupeLicenseAttempts(attempts) {
		result, err := c.postFreeLicense(ctx, &client, attempt, form, appID, packageID)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return FreeLicenseResult{}, lastErr
}

func (c *Client) postFreeLicense(ctx context.Context, client *http.Client, attempt freeLicenseAttempt, form url.Values, appID string, packageID int64) (FreeLicenseResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, attempt.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return FreeLicenseResult{}, err
	}
	req.Header = c.apiHeaders()
	req.Header.Set("Origin", "https://store.steampowered.com")
	req.Header.Set("Referer", "https://store.steampowered.com/app/"+appID+"/")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	if attempt.Ajax {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	resp, err := client.Do(req)
	if err != nil {
		return FreeLicenseResult{}, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return FreeLicenseResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FreeLicenseResult{}, fmt.Errorf("%s returned HTTP %d", attempt.Endpoint, resp.StatusCode)
	}

	body := strings.TrimSpace(string(content))
	if isFreeLicenseSuccessPage(body) || c.verifyOwnedApp(ctx, client, appID) == nil {
		return FreeLicenseResult{}, nil
	}

	lowerBody := strings.ToLower(body)
	if strings.Contains(lowerBody, "already") && (strings.Contains(lowerBody, "own") || strings.Contains(lowerBody, "have")) {
		return FreeLicenseResult{AlreadyOwned: true}, nil
	}
	if strings.Contains(body, "已拥有") || strings.Contains(body, "已经拥有") {
		return FreeLicenseResult{AlreadyOwned: true}, nil
	}
	if body == "" || body == "[]" {
		return FreeLicenseResult{}, fmt.Errorf("%s returned an empty response and dynamicstore did not show app %s as owned", attempt.Endpoint, appID)
	}
	return FreeLicenseResult{}, fmt.Errorf("%s: %s", attempt.Endpoint, extractHTMLFailure(body))
}

func isFreeLicenseSuccessPage(body string) bool {
	text := strings.ToLower(cleanHTMLText(stripTags(body)))
	return strings.Contains(text, "success!") ||
		strings.Contains(text, "success") && strings.Contains(text, "steam account") ||
		strings.Contains(text, "成功") && strings.Contains(text, "steam 帐户") ||
		strings.Contains(text, "成功") && strings.Contains(text, "steam 账户") ||
		strings.Contains(text, "已被绑定至您的 steam")
}

func dedupeLicenseAttempts(attempts []freeLicenseAttempt) []freeLicenseAttempt {
	seen := map[string]bool{}
	result := make([]freeLicenseAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		key := attempt.Endpoint
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, attempt)
	}
	return result
}

func (c *Client) verifyOwnedApp(ctx context.Context, client *http.Client, appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errors.New("app id is required to verify ownership")
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://store.steampowered.com/dynamicstore/userdata/?t="+fmt.Sprintf("%d", time.Now().UnixNano()), nil)
		if err != nil {
			return err
		}
		req.Header = c.apiHeaders()
		req.Header.Set("Origin", "https://store.steampowered.com")
		req.Header.Set("Referer", "https://store.steampowered.com/app/"+url.PathEscape(appID)+"/")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("dynamicstore userdata HTTP %d", resp.StatusCode)
		}
		if strings.Contains(strings.ToLower(resp.Request.URL.String()), "/login") {
			return errors.New("dynamicstore userdata redirected to login")
		}

		var payload struct {
			OwnedApps []int64 `json:"rgOwnedApps"`
		}
		if err := json.Unmarshal(content, &payload); err != nil {
			return err
		}
		for _, owned := range payload.OwnedApps {
			if fmt.Sprintf("%d", owned) == appID {
				return nil
			}
		}
		lastErr = fmt.Errorf("dynamicstore userdata did not show app %s as owned", appID)
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("dynamicstore userdata did not show app %s as owned", appID)
}

func (c *Client) freeLicenseForm(ctx context.Context, client *http.Client, appID string, packageID int64) (url.Values, string, error) {
	storeURL := "https://store.steampowered.com/app/" + url.PathEscape(appID) + "/?l=english"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, storeURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header = c.apiHeaders()
	req.Header.Set("Origin", "https://store.steampowered.com")
	req.Header.Set("Referer", "https://store.steampowered.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("free license form HTTP %d", resp.StatusCode)
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, "", err
	}

	values, action := findFreeLicenseForm(doc, packageID)
	if action == "" {
		return nil, "", fmt.Errorf("free license form for package %d was not found", packageID)
	}
	return values, action, nil
}

func findFreeLicenseForm(root *html.Node, packageID int64) (url.Values, string) {
	if root == nil {
		return nil, ""
	}
	formName := "add_to_cart_" + fmt.Sprintf("%d", packageID)
	var walk func(*html.Node) (*html.Node, bool)
	walk = func(node *html.Node) (*html.Node, bool) {
		if node.Type == html.ElementNode && node.Data == "form" && attr(node, "name") == formName {
			return node, true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if found, ok := walk(child); ok {
				return found, true
			}
		}
		return nil, false
	}

	form, ok := walk(root)
	if !ok {
		return nil, ""
	}
	values := url.Values{}
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			name := attr(node, "name")
			if name != "" {
				values.Set(name, attr(node, "value"))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(form)
	return values, attr(form, "action")
}

func (c *Client) verifyFreeLicenseOwned(ctx context.Context, client *http.Client, packageID int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://store.steampowered.com/account/licenses/", nil)
	if err != nil {
		return err
	}
	req.Header = c.apiHeaders()
	req.Header.Set("Origin", "https://store.steampowered.com")
	req.Header.Set("Referer", "https://store.steampowered.com/account/licenses/")

	var lastBody string
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		content, readErr := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("license verification HTTP %d", resp.StatusCode)
		}
		lastBody = string(content)
		id := fmt.Sprintf("%d", packageID)
		if strings.Contains(lastBody, "RemoveFreeLicense( "+id+",") ||
			strings.Contains(lastBody, "RemoveFreeLicense("+id+",") ||
			strings.Contains(lastBody, "packageid="+id) {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if strings.Contains(strings.ToLower(lastBody), "/login") {
		return errors.New("license verification redirected to login")
	}
	return fmt.Errorf("license verification did not find package %d", packageID)
}

func extractHTMLFailure(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "add free license returned an empty response"
	}
	if match := between(body, "<span>", "</span>"); match != "" {
		return cleanHTMLText(match)
	}
	if match := between(body, "<h2>", "</h2>"); match != "" {
		return cleanHTMLText(match)
	}
	return "add free license returned an unexpected response: " + responseSnippet(body)
}

func between(value string, start string, end string) string {
	left := strings.Index(value, start)
	if left < 0 {
		return ""
	}
	left += len(start)
	right := strings.Index(value[left:], end)
	if right < 0 {
		return ""
	}
	return value[left : left+right]
}

func cleanHTMLText(value string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func responseSnippet(value string) string {
	text := cleanHTMLText(stripTags(value))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func stripTags(value string) string {
	doc, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return value
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return builder.String()
}

func attr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, item := range node.Attr {
		if item.Key == name {
			return item.Val
		}
	}
	return ""
}

func jarHasCookie(jar *cookiejar.Jar, domain string, name string) bool {
	return cookieValue(jar, domain, name) != ""
}

func cookieValue(jar *cookiejar.Jar, domain string, name string) string {
	if jar == nil {
		return ""
	}
	target, err := url.Parse("https://" + domain + "/")
	if err != nil {
		return ""
	}
	for _, cookie := range jar.Cookies(target) {
		if cookie.Name == name && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}

func setResponseCookies(jar *cookiejar.Jar, resp *http.Response, domains map[string]bool) {
	if len(resp.Cookies()) == 0 {
		return
	}
	jar.SetCookies(resp.Request.URL, resp.Cookies())
	if host := resp.Request.URL.Hostname(); host != "" {
		domains[host] = true
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Domain != "" {
			domains[strings.TrimPrefix(cookie.Domain, ".")] = true
		}
	}
}

func randomSessionID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "steamscope-session"
	}
	return hex.EncodeToString(buf[:])
}

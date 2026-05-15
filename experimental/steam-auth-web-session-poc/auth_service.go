package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"steam-auth-web-session-poc/internal/sessionstore"
	"steam-auth-web-session-poc/internal/steamauth"
	"steam-auth-web-session-poc/internal/vault"
)

const (
	statusWaitingQRScan             = "waiting_qr_scan"
	statusWaitingGuardCode          = "waiting_guard_code"
	statusWaitingDeviceConfirmation = "waiting_device_confirmation"
	statusWaitingEmailConfirmation  = "waiting_email_confirmation"
	statusRemoteInteraction         = "remote_interaction"
	statusPolling                   = "polling"
	statusAuthenticated             = "authenticated"
	statusFailed                    = "failed"
	statusTimeout                   = "timeout"
	statusCanceled                  = "canceled"
)

type AuthService struct {
	client   *steamauth.Client
	store    *sessionstore.Store
	vault    vault.Vault
	initErr  error
	mu       sync.Mutex
	sessions map[string]*loginAttempt
}

type loginAttempt struct {
	LoginID          string
	ClientID         uint64
	RequestID        []byte
	SteamID          string
	Account          string
	Status           string
	ValidActions     []GuardAction
	PollIntervalSecs int
	ExpiresAt        time.Time
	Message          string
}

func NewAuthService(client *steamauth.Client, store *sessionstore.Store, tokenVault vault.Vault, initErr ...error) *AuthService {
	var err error
	if len(initErr) > 0 {
		err = initErr[0]
	}
	return &AuthService{
		client:   client,
		store:    store,
		vault:    tokenVault,
		initErr:  err,
		sessions: map[string]*loginAttempt{},
	}
}

func (s *AuthService) Ready() error {
	if s.initErr != nil {
		return s.initErr
	}
	if s.client == nil || s.store == nil || s.vault == nil {
		return errors.New("auth service dependencies are not ready")
	}
	return nil
}

func (s *AuthService) StartQRLogin(ctx context.Context) (*QRLoginStartResult, error) {
	resp, err := s.client.BeginQR(ctx)
	if err != nil {
		return nil, err
	}

	attempt := &loginAttempt{
		LoginID:          newLoginID(),
		ClientID:         resp.ClientID,
		RequestID:        resp.RequestID,
		Status:           statusWaitingQRScan,
		ValidActions:     mapGuardActions(resp.AllowedConfirmations),
		PollIntervalSecs: normalizePollInterval(resp.Interval),
		ExpiresAt:        time.Now().Add(3 * time.Minute),
		Message:          "等待使用 Steam 手机 App 扫码。",
	}

	s.mu.Lock()
	s.sessions[attempt.LoginID] = attempt
	s.mu.Unlock()

	return &QRLoginStartResult{
		LoginID:           attempt.LoginID,
		QRChallengeURL:    resp.ChallengeURL,
		Status:            attempt.Status,
		PollIntervalSecs:  attempt.PollIntervalSecs,
		ValidActions:      attempt.ValidActions,
		ExpiresAt:         attempt.ExpiresAt.Format(time.RFC3339),
		SafeStatusMessage: attempt.Message,
	}, nil
}

func (s *AuthService) StartCredentialLogin(ctx context.Context, req CredentialLoginStartRequest) (*LoginStartResult, error) {
	accountName := strings.TrimSpace(req.AccountName)
	if accountName == "" || req.Password == "" {
		return nil, errors.New("account name and password are required")
	}
	password := req.Password
	defer func() {
		password = ""
		req.Password = ""
	}()

	resp, err := s.client.BeginCredentials(ctx, accountName, password)
	if err != nil {
		return nil, err
	}

	actions := mapGuardActions(resp.AllowedConfirmations)
	status, message := statusForActions(actions)
	attempt := &loginAttempt{
		LoginID:          newLoginID(),
		ClientID:         resp.ClientID,
		RequestID:        resp.RequestID,
		SteamID:          resp.SteamID,
		Account:          accountName,
		Status:           status,
		ValidActions:     actions,
		PollIntervalSecs: normalizePollInterval(resp.Interval),
		ExpiresAt:        time.Now().Add(3 * time.Minute),
		Message:          message,
	}

	s.mu.Lock()
	s.sessions[attempt.LoginID] = attempt
	s.mu.Unlock()

	if len(actions) == 0 || status == statusPolling {
		_, _ = s.GetLoginStatus(ctx, attempt.LoginID)
	}

	return &LoginStartResult{
		LoginID:           attempt.LoginID,
		Status:            attempt.Status,
		PollIntervalSecs:  attempt.PollIntervalSecs,
		ValidActions:      attempt.ValidActions,
		ExpiresAt:         attempt.ExpiresAt.Format(time.RFC3339),
		SafeStatusMessage: attempt.Message,
	}, nil
}

func (s *AuthService) SubmitGuardCode(ctx context.Context, req SubmitGuardCodeRequest) error {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return errors.New("guard code is required")
	}

	s.mu.Lock()
	attempt, ok := s.sessions[req.LoginID]
	s.mu.Unlock()
	if !ok {
		return errors.New("login attempt not found")
	}

	guardType, err := steamauth.GuardTypeFromString(req.Type)
	if err != nil {
		return err
	}

	if err := s.client.SubmitGuardCode(ctx, steamauth.SubmitGuardCodeRequest{
		ClientID: attempt.ClientID,
		SteamID:  attempt.SteamID,
		Code:     code,
		CodeType: guardType,
	}); err != nil {
		return err
	}

	s.mu.Lock()
	attempt.Status = statusPolling
	attempt.Message = "验证码已提交，正在等待 Steam 完成登录。"
	s.mu.Unlock()
	return nil
}

func (s *AuthService) GetLoginStatus(ctx context.Context, loginID string) (*LoginStatus, error) {
	s.mu.Lock()
	attempt, ok := s.sessions[loginID]
	if !ok {
		s.mu.Unlock()
		return nil, errors.New("login attempt not found")
	}
	if time.Now().After(attempt.ExpiresAt) && attempt.Status != statusAuthenticated {
		attempt.Status = statusTimeout
		attempt.Message = "登录已超时，请重新发起登录。"
		status := attempt.toStatus()
		s.mu.Unlock()
		return status, nil
	}
	shouldPoll := attempt.Status == statusWaitingQRScan ||
		attempt.Status == statusWaitingDeviceConfirmation ||
		attempt.Status == statusWaitingEmailConfirmation ||
		attempt.Status == statusRemoteInteraction ||
		attempt.Status == statusPolling
	clientID := attempt.ClientID
	requestID := append([]byte(nil), attempt.RequestID...)
	s.mu.Unlock()

	if shouldPoll {
		poll, err := s.client.Poll(ctx, clientID, requestID)
		if err != nil {
			s.mu.Lock()
			attempt.Status = statusFailed
			attempt.Message = safeError(err)
			status := attempt.toStatus()
			s.mu.Unlock()
			return status, nil
		}

		s.mu.Lock()
		if poll.NewClientID != 0 {
			attempt.ClientID = poll.NewClientID
		}
		if poll.RefreshToken != "" {
			attempt.Status = statusAuthenticated
			attempt.Message = "登录成功，refresh token 已加密保存。"
			attempt.Account = poll.AccountName
			steamID := poll.SteamID()
			if steamID != "" {
				attempt.SteamID = steamID
			}
			if err := s.saveAuthenticatedLocked(attempt, poll.RefreshToken); err != nil {
				attempt.Status = statusFailed
				attempt.Message = safeError(err)
			}
		} else if poll.HadRemoteInteraction {
			attempt.Status = statusRemoteInteraction
			attempt.Message = "已检测到远端交互，请在 Steam 手机 App 或邮箱中确认。"
		}
		status := attempt.toStatus()
		s.mu.Unlock()
		return status, nil
	}

	s.mu.Lock()
	status := attempt.toStatus()
	s.mu.Unlock()
	return status, nil
}

func (s *AuthService) CancelLogin(loginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.sessions[loginID]
	if !ok {
		return errors.New("login attempt not found")
	}
	attempt.Status = statusCanceled
	attempt.Message = "登录已取消。"
	return nil
}

func (s *AuthService) GetAccount() (*SteamAccountSummary, error) {
	account, err := s.store.Load()
	if errors.Is(err, sessionstore.ErrNoAccount) {
		return &SteamAccountSummary{LoggedIn: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return &SteamAccountSummary{
		SteamID:     account.SteamID,
		Account:     account.AccountName,
		LoggedIn:    true,
		LastLoginAt: account.LastLoginAt,
	}, nil
}

func (s *AuthService) Logout() error {
	account, err := s.store.Load()
	if err != nil && !errors.Is(err, sessionstore.ErrNoAccount) {
		return err
	}
	if account.RefreshTokenRef != "" {
		_ = s.vault.Delete(account.RefreshTokenRef)
	}
	return s.store.Delete()
}

func (s *AuthService) TestWebSession(ctx context.Context) (*WebSessionTestResult, error) {
	account, err := s.store.Load()
	if err != nil {
		if errors.Is(err, sessionstore.ErrNoAccount) {
			return &WebSessionTestResult{OK: false, Message: "尚未登录。"}, nil
		}
		return nil, err
	}

	refreshToken, err := s.vault.Get(account.RefreshTokenRef)
	if err != nil {
		return nil, err
	}

	jar, domains, err := s.client.GetWebCookieJar(ctx, refreshToken)
	if err != nil {
		return &WebSessionTestResult{OK: false, SteamID: account.SteamID, Account: account.AccountName, Message: safeError(err)}, nil
	}

	if err := s.client.TestCommunitySession(ctx, jar, account.SteamID); err != nil {
		return &WebSessionTestResult{OK: false, SteamID: account.SteamID, Account: account.AccountName, CookieDomains: domains, CommunityOK: false, StoreOK: false, Message: safeError(err)}, nil
	}
	if err := s.client.TestStoreSession(ctx, jar, account.SteamID); err != nil {
		return &WebSessionTestResult{OK: false, SteamID: account.SteamID, Account: account.AccountName, CookieDomains: domains, CommunityOK: true, StoreOK: false, Message: safeError(err)}, nil
	}

	now := time.Now().Format(time.RFC3339)
	account.LastCookieRefreshAt = now
	_ = s.store.Save(account)

	return &WebSessionTestResult{
		OK:                  true,
		SteamID:             account.SteamID,
		Account:             account.AccountName,
		CookieDomains:       domains,
		CommunityOK:         true,
		StoreOK:             true,
		LastCookieRefreshAt: now,
		Message:             "Community 和 Store Web Cookie 已在 Go 后端验证可用，未返回给前端。",
	}, nil
}

func (s *AuthService) ClaimFreeLicense(ctx context.Context, appID string, packageID int64) (steamauth.FreeLicenseResult, int, error) {
	account, err := s.store.Load()
	if err != nil {
		if errors.Is(err, sessionstore.ErrNoAccount) {
			return steamauth.FreeLicenseResult{}, 0, errors.New("尚未登录。")
		}
		return steamauth.FreeLicenseResult{}, 0, err
	}

	refreshToken, err := s.vault.Get(account.RefreshTokenRef)
	if err != nil {
		return steamauth.FreeLicenseResult{}, 0, err
	}

	jar, domains, err := s.client.GetWebCookieJar(ctx, refreshToken)
	if err != nil {
		return steamauth.FreeLicenseResult{}, domains, err
	}
	if err := s.client.TestCommunitySession(ctx, jar, account.SteamID); err != nil {
		return steamauth.FreeLicenseResult{}, domains, err
	}
	if err := s.client.TestStoreSession(ctx, jar, account.SteamID); err != nil {
		return steamauth.FreeLicenseResult{}, domains, err
	}
	result, err := s.client.AddFreeLicense(ctx, jar, appID, packageID)
	if err != nil {
		return result, domains, err
	}

	account.LastCookieRefreshAt = time.Now().Format(time.RFC3339)
	_ = s.store.Save(account)
	return result, domains, nil
}

func (s *AuthService) saveAuthenticatedLocked(attempt *loginAttempt, refreshToken string) error {
	if attempt.SteamID == "" {
		attempt.SteamID = steamauth.SteamIDFromJWT(refreshToken)
	}
	if attempt.Account == "" {
		attempt.Account = "unknown"
	}
	ref := "steam-web-refresh-token"
	if err := s.vault.Put(ref, refreshToken); err != nil {
		return err
	}
	return s.store.Save(sessionstore.Account{
		SteamID:             attempt.SteamID,
		AccountName:         attempt.Account,
		RefreshTokenRef:     ref,
		PlatformType:        "web_browser",
		LastLoginAt:         time.Now().Format(time.RFC3339),
		LastCookieRefreshAt: "",
	})
}

func (a *loginAttempt) toStatus() *LoginStatus {
	return &LoginStatus{
		LoginID:           a.LoginID,
		Status:            a.Status,
		SteamID:           a.SteamID,
		Account:           a.Account,
		PollIntervalSecs:  a.PollIntervalSecs,
		ValidActions:      a.ValidActions,
		ExpiresAt:         a.ExpiresAt.Format(time.RFC3339),
		SafeStatusMessage: a.Message,
	}
}

func mapGuardActions(confirmations []steamauth.AllowedConfirmation) []GuardAction {
	actions := make([]GuardAction, 0, len(confirmations))
	for _, item := range confirmations {
		guardType := steamauth.GuardTypeName(item.Type)
		if guardType == "none" || guardType == "unknown" {
			continue
		}
		actions = append(actions, GuardAction{
			Type:   guardType,
			Detail: item.Message,
		})
	}
	return actions
}

func statusForActions(actions []GuardAction) (string, string) {
	if len(actions) == 0 {
		return statusPolling, "凭据已提交，正在等待 Steam 完成登录。"
	}
	for _, action := range actions {
		switch action.Type {
		case "device_confirmation":
			return statusWaitingDeviceConfirmation, "请在 Steam 手机 App 中确认登录。"
		case "email_confirmation":
			return statusWaitingEmailConfirmation, "请在邮箱中确认登录。"
		}
	}
	for _, action := range actions {
		switch action.Type {
		case "email_code", "device_code":
			return statusWaitingGuardCode, "需要输入 Steam Guard 验证码。"
		}
	}
	return statusPolling, "正在等待 Steam 完成登录。"
}

func normalizePollInterval(interval float32) int {
	if interval <= 0 {
		return 2
	}
	if interval < 1 {
		return 1
	}
	if interval > 10 {
		return 10
	}
	return int(interval + 0.5)
}

func newLoginID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("login-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	blocked := []string{"password", "refresh_token", "access_token", "steamLoginSecure", "sessionid", "Set-Cookie"}
	for _, term := range blocked {
		msg = strings.ReplaceAll(msg, term, "[redacted]")
	}
	return msg
}

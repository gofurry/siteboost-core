package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"steam-auth-web-session-poc/internal/sessionstore"
	"steam-auth-web-session-poc/internal/steamauth"
	"steam-auth-web-session-poc/internal/vault"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	service      *AuthService
	sessionStore *sessionstore.Store
	tokenVault   vault.Vault
	steamClient  *http.Client
	networkStore *NetworkConfigStore
	networkCfg   NetworkConfig
	networkErr   error
	freebieStore *FreebieStore
	freebieErr   error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, err := sessionstore.New()
	if err != nil {
		a.service = NewAuthService(nil, nil, nil, err)
		return
	}
	a.sessionStore = store

	tokenVault, err := vault.New()
	if err != nil {
		a.service = NewAuthService(nil, nil, nil, err)
		return
	}
	a.tokenVault = tokenVault

	a.networkStore, a.networkErr = NewNetworkConfigStore()
	if a.networkErr == nil {
		a.networkCfg, a.networkErr = a.networkStore.Load()
	}
	if err := a.rebuildSteamClient(); err != nil {
		a.service = NewAuthService(nil, nil, nil, err)
		return
	}

	client := steamauth.NewClient(a.steamClient, steamauth.DefaultUserAgent)
	a.service = NewAuthService(client, store, tokenVault)

	a.freebieStore, a.freebieErr = NewFreebieStore()
}

func (a *App) StartSteamQRLogin() (*QRLoginStartResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.service.StartQRLogin(context.Background())
}

func (a *App) StartSteamCredentialLogin(req CredentialLoginStartRequest) (*LoginStartResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.service.StartCredentialLogin(context.Background(), req)
}

func (a *App) SubmitSteamGuardCode(req SubmitGuardCodeRequest) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.service.SubmitGuardCode(context.Background(), req)
}

func (a *App) CancelSteamLogin(loginID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.service.CancelLogin(loginID)
}

func (a *App) GetSteamLoginStatus(loginID string) (*LoginStatus, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.service.GetLoginStatus(ctx, loginID)
}

func (a *App) GetSteamAccount() (*SteamAccountSummary, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.service.GetAccount()
}

func (a *App) LogoutSteam() error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.service.Logout()
}

func (a *App) TestWebSession() (*WebSessionTestResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.service.TestWebSession(ctx)
}

func (a *App) GetNetworkConfig() (*NetworkConfig, error) {
	if err := a.networkReady(); err != nil {
		return nil, err
	}
	cfg := a.networkCfg
	return &cfg, nil
}

func (a *App) SaveNetworkConfig(req NetworkConfigRequest) (*NetworkConfig, error) {
	if err := a.networkReady(); err != nil {
		return nil, err
	}
	cfg := NetworkConfig{ProxyURL: strings.TrimSpace(req.ProxyURL)}
	if err := a.networkStore.Save(cfg); err != nil {
		return nil, err
	}
	a.networkCfg = cfg
	if err := a.rebuildSteamClient(); err != nil {
		return nil, err
	}
	a.service = NewAuthService(steamauth.NewClient(a.steamClient, steamauth.DefaultUserAgent), a.sessionStore, a.tokenVault)
	return &cfg, nil
}

func (a *App) RefreshFreebies() (*FreebieSnapshot, error) {
	if err := a.freebiesReady(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	source := NewSteamSearchSource(a.httpClient())
	items, err := source.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.freebieStore.UpsertFetched(items, time.Now()); err != nil {
		return nil, err
	}
	return a.freebieStore.Snapshot(), nil
}

func (a *App) ListFreebies() (*FreebieSnapshot, error) {
	if err := a.freebiesReady(); err != nil {
		return nil, err
	}
	return a.freebieStore.Snapshot(), nil
}

func (a *App) MarkFreebieStatus(appID string, status string, note string) (*FreebieSnapshot, error) {
	if err := a.freebiesReady(); err != nil {
		return nil, err
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, errors.New("appID is required")
	}
	if !isKnownStatus(status) {
		return nil, fmt.Errorf("unsupported status %q", status)
	}
	if err := a.freebieStore.MarkStatus(appID, status, note, time.Now()); err != nil {
		return nil, err
	}
	return a.freebieStore.Snapshot(), nil
}

func (a *App) ClaimFreebie(appID string) (*FreebieClaimResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := a.freebiesReady(); err != nil {
		return nil, err
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, errors.New("appID is required")
	}

	item, ok := a.freebieStore.Get(appID)
	if !ok {
		return nil, errors.New("freebie is not in the local list")
	}
	if item.PackageID == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		packageID, packageTitle, err := NewSteamSearchSource(a.httpClient()).ResolveFreePackage(ctx, appID)
		if err != nil {
			_ = a.freebieStore.MarkStatus(appID, freebieStatusFailed, err.Error(), time.Now())
			return &FreebieClaimResult{OK: false, AppID: appID, Message: safeError(err), Snapshot: a.freebieStore.Snapshot()}, nil
		}
		item.PackageID = packageID
		item.PackageTitle = packageTitle
		_ = a.freebieStore.UpsertOne(item, time.Now())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	result, domains, err := a.service.ClaimFreeLicense(ctx, appID, item.PackageID)
	if err != nil {
		_ = a.freebieStore.MarkStatus(appID, freebieStatusFailed, safeError(err), time.Now())
		return &FreebieClaimResult{OK: false, AppID: appID, PackageID: item.PackageID, CookieDomains: domains, Message: safeError(err), Snapshot: a.freebieStore.Snapshot()}, nil
	}

	message := "领取成功，已标记为已入库。"
	if result.AlreadyOwned {
		message = "Steam 返回已拥有，已标记为已入库。"
	}
	if err := a.freebieStore.MarkStatus(appID, freebieStatusClaimed, message, time.Now()); err != nil {
		return nil, err
	}
	return &FreebieClaimResult{
		OK:            true,
		AppID:         appID,
		PackageID:     item.PackageID,
		CookieDomains: domains,
		Message:       message,
		Snapshot:      a.freebieStore.Snapshot(),
	}, nil
}

func (a *App) OpenStorePage(appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errors.New("appID is required")
	}
	if a.ctx == nil {
		return errors.New("wails context is not ready")
	}
	runtime.BrowserOpenURL(a.ctx, "https://store.steampowered.com/app/"+url.PathEscape(appID))
	return nil
}

func (a *App) ready() error {
	if a.service == nil {
		return errors.New("auth service is not initialized")
	}
	return a.service.Ready()
}

func (a *App) networkReady() error {
	if a.networkErr != nil {
		return a.networkErr
	}
	if a.networkStore == nil {
		return errors.New("network config store is not initialized")
	}
	return nil
}

func (a *App) rebuildSteamClient() error {
	client, err := NewHTTPClient(a.networkCfg.ProxyURL)
	if err != nil {
		return err
	}
	a.steamClient = client
	return nil
}

func (a *App) httpClient() *http.Client {
	if a.steamClient != nil {
		return a.steamClient
	}
	return http.DefaultClient
}

func (a *App) freebiesReady() error {
	if a.freebieErr != nil {
		return a.freebieErr
	}
	if a.freebieStore == nil {
		return errors.New("freebie store is not initialized")
	}
	return nil
}

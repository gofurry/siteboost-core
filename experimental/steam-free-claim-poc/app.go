package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	statusTodo    = "todo"
	statusClaimed = "claimed"
	statusSkipped = "skipped"
	statusFailed  = "failed"
)

// App exposes the Steam free-to-keep proof-of-concept workflow to Wails.
type App struct {
	ctx   context.Context
	store *Store
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, err := NewStore()
	if err != nil {
		runtime.LogErrorf(ctx, "init local store: %v", err)
		return
	}
	a.store = store
}

func (a *App) RefreshFreebies() (*FreebieSnapshot, error) {
	if a.store == nil {
		return nil, errors.New("local store is not ready")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	source := NewSteamSearchSource(http.DefaultClient)
	items, err := source.Fetch(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := a.store.UpsertFetched(items, now); err != nil {
		return nil, err
	}

	return a.store.Snapshot(), nil
}

func (a *App) ListFreebies() (*FreebieSnapshot, error) {
	if a.store == nil {
		return nil, errors.New("local store is not ready")
	}
	return a.store.Snapshot(), nil
}

func (a *App) MarkFreebieStatus(appID string, status string, note string) (*FreebieSnapshot, error) {
	if a.store == nil {
		return nil, errors.New("local store is not ready")
	}
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("appID is required")
	}
	if !isKnownStatus(status) {
		return nil, fmt.Errorf("unsupported status %q", status)
	}

	if err := a.store.MarkStatus(appID, status, note, time.Now()); err != nil {
		return nil, err
	}
	return a.store.Snapshot(), nil
}

func (a *App) OpenStorePage(appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errors.New("appID is required")
	}
	return a.openURL("https://store.steampowered.com/app/" + url.PathEscape(appID))
}

func (a *App) OpenSteamLogin() error {
	return a.openURL("https://store.steampowered.com/login/")
}

func (a *App) OpenSteamSearch() error {
	return a.openURL(steamSearchURL)
}

func (a *App) openURL(target string) error {
	if a.ctx == nil {
		return errors.New("wails context is not ready")
	}
	runtime.BrowserOpenURL(a.ctx, target)
	return nil
}

func isKnownStatus(status string) bool {
	switch status {
	case statusTodo, statusClaimed, statusSkipped, statusFailed:
		return true
	default:
		return false
	}
}

func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "SteamScope", "steam-free-claim-poc", "state.json"), nil
}

func sortFreebies(items []Freebie) {
	sort.SliceStable(items, func(i, j int) bool {
		leftStatus := statusWeight(items[i].Status)
		rightStatus := statusWeight(items[j].Status)
		if leftStatus != rightStatus {
			return leftStatus < rightStatus
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
}

func statusWeight(status string) int {
	switch status {
	case statusTodo:
		return 0
	case statusFailed:
		return 1
	case statusSkipped:
		return 2
	case statusClaimed:
		return 3
	default:
		return 4
	}
}

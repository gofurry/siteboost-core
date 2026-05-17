package sessionstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrNoAccount = errors.New("no stored steam account")

type Account struct {
	SteamID             string `json:"steamId"`
	AccountName         string `json:"accountName"`
	RefreshTokenRef     string `json:"refreshTokenRef"`
	PlatformType        string `json:"platformType"`
	LastLoginAt         string `json:"lastLoginAt"`
	LastCookieRefreshAt string `json:"lastCookieRefreshAt"`
}

type Store struct {
	path string
}

func New() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(base, "SteamScope", "steam-auth-web-session-poc", "account.json")}, nil
}

func (s *Store) Load() (Account, error) {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Account{}, ErrNoAccount
	}
	if err != nil {
		return Account{}, err
	}
	var account Account
	if err := json.Unmarshal(content, &account); err != nil {
		return Account{}, err
	}
	if account.RefreshTokenRef == "" {
		return Account{}, ErrNoAccount
	}
	return account, nil
}

func (s *Store) Save(account Account) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0o600)
}

func (s *Store) Delete() error {
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

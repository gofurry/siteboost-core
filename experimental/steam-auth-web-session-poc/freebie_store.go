package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	freebieStatusTodo    = "todo"
	freebieStatusClaimed = "claimed"
	freebieStatusSkipped = "skipped"
	freebieStatusFailed  = "failed"
)

type FreebieStore struct {
	path  string
	state freebiePersistentState
}

type freebiePersistentState struct {
	Items         map[string]Freebie `json:"items"`
	LastRefreshAt string             `json:"lastRefreshAt"`
}

func NewFreebieStore() (*FreebieStore, error) {
	path, err := freebieConfigPath()
	if err != nil {
		return nil, err
	}
	store := &FreebieStore{
		path: path,
		state: freebiePersistentState{
			Items: map[string]Freebie{},
		},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FreebieStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}
	if err := json.Unmarshal(content, &s.state); err != nil {
		return err
	}
	if s.state.Items == nil {
		s.state.Items = map[string]Freebie{}
	}
	return nil
}

func (s *FreebieStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0o600)
}

func (s *FreebieStore) UpsertFetched(items []Freebie, now time.Time) error {
	nowText := now.Format(time.RFC3339)
	for _, item := range items {
		if item.AppID == "" {
			continue
		}
		existing, ok := s.state.Items[item.AppID]
		if ok {
			item.Status = existing.Status
			item.Note = existing.Note
			item.FirstSeenAt = existing.FirstSeenAt
			item.UpdatedAt = existing.UpdatedAt
			if item.PackageID == 0 {
				item.PackageID = existing.PackageID
				item.PackageTitle = existing.PackageTitle
			}
		}
		if item.Status == "" {
			item.Status = freebieStatusTodo
		}
		if item.FirstSeenAt == "" {
			item.FirstSeenAt = nowText
		}
		item.LastSeenAt = nowText
		s.state.Items[item.AppID] = item
	}
	s.state.LastRefreshAt = nowText
	return s.save()
}

func (s *FreebieStore) Get(appID string) (Freebie, bool) {
	item, ok := s.state.Items[appID]
	return item, ok
}

func (s *FreebieStore) MarkStatus(appID string, status string, note string, now time.Time) error {
	item, ok := s.state.Items[appID]
	if !ok {
		return errors.New("freebie is not in the local list")
	}
	item.Status = status
	item.Note = note
	item.UpdatedAt = now.Format(time.RFC3339)
	s.state.Items[appID] = item
	return s.save()
}

func (s *FreebieStore) UpsertOne(item Freebie, now time.Time) error {
	if item.AppID == "" {
		return errors.New("appID is required")
	}
	nowText := now.Format(time.RFC3339)
	existing, ok := s.state.Items[item.AppID]
	if ok {
		if item.Status == "" {
			item.Status = existing.Status
		}
		item.Note = existing.Note
		item.FirstSeenAt = existing.FirstSeenAt
		item.UpdatedAt = existing.UpdatedAt
	}
	if item.Status == "" {
		item.Status = freebieStatusTodo
	}
	if item.FirstSeenAt == "" {
		item.FirstSeenAt = nowText
	}
	item.LastSeenAt = nowText
	s.state.Items[item.AppID] = item
	return s.save()
}

func (s *FreebieStore) Snapshot() *FreebieSnapshot {
	items := make([]Freebie, 0, len(s.state.Items))
	snapshot := &FreebieSnapshot{
		LastRefreshAt: s.state.LastRefreshAt,
		SourceURL:     steamSearchURL,
	}
	for _, item := range s.state.Items {
		items = append(items, item)
		switch item.Status {
		case freebieStatusClaimed:
			snapshot.ClaimedCount++
		case freebieStatusSkipped:
			snapshot.SkippedCount++
		case freebieStatusFailed:
			snapshot.FailedCount++
		default:
			snapshot.TodoCount++
		}
	}
	sortFreebies(items)
	snapshot.Items = items
	snapshot.Total = len(items)
	return snapshot
}

func freebieConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "SteamScope", "steam-auth-web-session-poc", "freebies.json"), nil
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
	case freebieStatusTodo:
		return 0
	case freebieStatusFailed:
		return 1
	case freebieStatusSkipped:
		return 2
	case freebieStatusClaimed:
		return 3
	default:
		return 4
	}
}

func isKnownStatus(status string) bool {
	switch status {
	case freebieStatusTodo, freebieStatusClaimed, freebieStatusSkipped, freebieStatusFailed:
		return true
	default:
		return false
	}
}

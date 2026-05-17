package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	path  string
	state persistentState
}

type persistentState struct {
	Items         map[string]Freebie `json:"items"`
	LastRefreshAt string             `json:"lastRefreshAt"`
}

func NewStore() (*Store, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	store := &Store{
		path: path,
		state: persistentState{
			Items: map[string]Freebie{},
		},
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) load() error {
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

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0o600)
}

func (s *Store) UpsertFetched(items []Freebie, now time.Time) error {
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
		}
		if item.Status == "" {
			item.Status = statusTodo
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

func (s *Store) MarkStatus(appID string, status string, note string, now time.Time) error {
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

func (s *Store) Snapshot() *FreebieSnapshot {
	items := make([]Freebie, 0, len(s.state.Items))
	snapshot := &FreebieSnapshot{
		LastRefreshAt: s.state.LastRefreshAt,
		SourceURL:     steamSearchURL,
	}

	for _, item := range s.state.Items {
		items = append(items, item)
		switch item.Status {
		case statusClaimed:
			snapshot.ClaimedCount++
		case statusSkipped:
			snapshot.SkippedCount++
		case statusFailed:
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

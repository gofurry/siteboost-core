package runtimecontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "runtime.json")
	state := State{
		PID:        123,
		Mode:       "proxy_only",
		ProxyAddr:  "127.0.0.1:26501",
		ControlURL: "http://127.0.0.1:1",
		Token:      "token",
		StartedAt:  time.Now().UTC(),
	}
	if err := WriteState(path, state); err != nil {
		t.Fatal(err)
	}
	got, err := ReadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != state.PID || got.Token != state.Token {
		t.Fatalf("state = %#v", got)
	}
	if err := RemoveState(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state file still exists: %v", err)
	}
}

func TestControlServerAuthStatusAndStop(t *testing.T) {
	token := "test-token"
	stopped := make(chan struct{})
	server, err := NewControlServer("127.0.0.1:0", token, func() any {
		return map[string]any{"running": true}
	}, func() {
		close(stopped)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})

	resp, err := http.Get(server.URL() + "/status")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL()+"/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body["running"] {
		t.Fatalf("running = false")
	}

	req, err = http.NewRequest(http.MethodPost, server.URL()+"/stop", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("stop status = %d", resp.StatusCode)
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatalf("stop callback was not called")
	}
}

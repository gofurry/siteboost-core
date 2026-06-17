package engine

import (
	"context"
	"testing"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
)

func TestEngineStartStopStatus(t *testing.T) {
	cfg := config.Default()
	cfg.Proxy.ListenAddr = "127.0.0.1:0"
	cfg.Runtime.StatePath = t.TempDir() + "/runtime.json"

	eng, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	status := eng.Status()
	if !status.Running {
		t.Fatalf("engine should be running")
	}
	if status.ListenAddr == "" {
		t.Fatalf("listen addr is empty")
	}
	if status.RuleCount == 0 {
		t.Fatalf("rule count is zero")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := eng.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if eng.Status().Running {
		t.Fatalf("engine should be stopped")
	}
}

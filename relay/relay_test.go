// relay/relay_test.go
package relay

import (
	"context"
	"testing"
	"time"
)

func TestRelayNewRejectsEmptyInclude(t *testing.T) {
	_, err := New(&Config{})
	if err == nil {
		t.Error("expected error for empty Include")
	}
}

func TestRelayNewAppliesDefaults(t *testing.T) {
	r, err := New(&Config{Include: []string{"lo"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r.cfg.WatchInterval != 5*time.Second {
		t.Errorf("WatchInterval default not applied: %v", r.cfg.WatchInterval)
	}
}

func TestRelayStartShutdownOnContextCancel(t *testing.T) {
	r, err := New(&Config{
		Include:       []string{"lo"},
		WatchInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Start(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Start returned %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of cancel")
	}
}

func TestRelayStatsAfterStartup(t *testing.T) {
	r, err := New(&Config{Include: []string{"lo"}, WatchInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)
	s := r.Stats()
	// We only assert the call works and doesn't race; value may be 0 on
	// systems where lo doesn't accept multicast joins.
	_ = s
}

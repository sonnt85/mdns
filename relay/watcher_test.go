package relay

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeLister lets tests inject a synthetic interface list.
type fakeLister struct {
	mu     sync.Mutex
	ifaces []net.Interface
}

func (f *fakeLister) set(ifaces []net.Interface) {
	f.mu.Lock()
	f.ifaces = append([]net.Interface(nil), ifaces...)
	f.mu.Unlock()
}

func (f *fakeLister) list() ([]net.Interface, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]net.Interface(nil), f.ifaces...), nil
}

func TestPollingWatcherInitialCall(t *testing.T) {
	fl := &fakeLister{}
	fl.set([]net.Interface{{Index: 1, Name: "eth0"}})

	w := &pollingWatcher{interval: 10 * time.Millisecond, list: fl.list}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan []net.Interface, 4)
	go func() { _ = w.Start(ctx, func(ifaces []net.Interface) { got <- ifaces }) }()

	select {
	case ifaces := <-got:
		if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
			t.Errorf("first callback = %+v, want [eth0]", ifaces)
		}
	case <-time.After(time.Second):
		t.Fatal("no initial callback within 1s")
	}
}

func TestPollingWatcherDetectsChange(t *testing.T) {
	fl := &fakeLister{}
	fl.set([]net.Interface{{Index: 1, Name: "eth0"}})

	w := &pollingWatcher{interval: 10 * time.Millisecond, list: fl.list}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan []net.Interface, 8)
	go func() { _ = w.Start(ctx, func(ifaces []net.Interface) { got <- ifaces }) }()

	// drain initial
	<-got

	// add iface
	fl.set([]net.Interface{{Index: 1, Name: "eth0"}, {Index: 2, Name: "docker0"}})

	// wait for change notification
	deadline := time.After(time.Second)
	for {
		select {
		case ifaces := <-got:
			if len(ifaces) == 2 {
				return
			}
		case <-deadline:
			t.Fatal("change not detected within 1s")
		}
	}
}

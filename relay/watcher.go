package relay

import (
	"context"
	"net"
	"time"
)

// pollingWatcher implements InterfaceWatcher by polling list() at interval.
type pollingWatcher struct {
	interval time.Duration
	list     func() ([]net.Interface, error) // injectable for tests; defaults to net.Interfaces
}

func newPollingWatcher(interval time.Duration) *pollingWatcher {
	return &pollingWatcher{interval: interval, list: net.Interfaces}
}

func (w *pollingWatcher) Start(ctx context.Context, onChange func([]net.Interface)) error {
	last := map[string]int{}

	fire := func() {
		ifaces, err := w.list()
		if err != nil {
			return
		}
		// detect set change by (name, index) pairs
		cur := make(map[string]int, len(ifaces))
		for _, i := range ifaces {
			cur[i.Name] = i.Index
		}
		if mapsEqual(cur, last) {
			return
		}
		last = cur
		onChange(ifaces)
	}

	// always fire once at startup
	fire()

	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			fire()
		}
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

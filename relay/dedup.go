package relay

import (
	"hash/fnv"
	"sync"
	"time"
)

// dedup is a lock-protected map from packet hash to first-seen timestamp.
// Entries older than window are treated as absent; expired entries are
// purged lazily on each seen() call.
//
// Cache key is FNV-64a(packet_bytes). Hash collisions (~2^-64) are
// acceptable: at worst a legitimate packet is dropped once within a 1s
// window, and mDNS retries handle this.
type dedup struct {
	window time.Duration
	mu     sync.Mutex
	seenAt map[uint64]time.Time
	last   time.Time // last purge time; bounds purge work
}

func newDedup(window time.Duration) *dedup {
	return &dedup{
		window: window,
		seenAt: make(map[uint64]time.Time),
	}
}

// seen reports whether pkt (by hash) was seen within the window.
// Always updates the last-seen timestamp.
func (d *dedup) seen(pkt []byte) bool {
	h := fnv.New64a()
	h.Write(pkt)
	key := h.Sum64()

	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	d.purgeLocked(now)

	prev, ok := d.seenAt[key]
	d.seenAt[key] = now
	return ok && now.Sub(prev) <= d.window
}

func (d *dedup) purgeLocked(now time.Time) {
	// Only purge at most once per window interval to cap O(n) work.
	if now.Sub(d.last) < d.window {
		return
	}
	d.last = now
	for k, t := range d.seenAt {
		if now.Sub(t) > d.window {
			delete(d.seenAt, k)
		}
	}
}

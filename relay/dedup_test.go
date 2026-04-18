package relay

import (
	"sync"
	"testing"
	"time"
)

func TestDedupSeenBasic(t *testing.T) {
	d := newDedup(100 * time.Millisecond)
	pkt := []byte("hello world packet")

	if d.seen(pkt) {
		t.Error("first seen() should return false")
	}
	if !d.seen(pkt) {
		t.Error("second seen() within window should return true")
	}
}

func TestDedupExpiry(t *testing.T) {
	d := newDedup(50 * time.Millisecond)
	pkt := []byte("expiring packet")

	d.seen(pkt)
	time.Sleep(80 * time.Millisecond)
	if d.seen(pkt) {
		t.Error("seen() after window expiry should return false")
	}
}

func TestDedupDifferentPackets(t *testing.T) {
	d := newDedup(time.Second)
	if d.seen([]byte("a")) {
		t.Error("first of A")
	}
	if d.seen([]byte("b")) {
		t.Error("first of B")
	}
	if !d.seen([]byte("a")) {
		t.Error("second of A")
	}
}

func TestDedupConcurrent(t *testing.T) {
	d := newDedup(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pkt := []byte{byte(id)}
			for j := 0; j < 1000; j++ {
				d.seen(pkt)
			}
		}(i)
	}
	wg.Wait()
	// No assertions — we're checking race detector doesn't complain.
}

// relay/stats.go
package relay

import "sync/atomic"

// Stats is a snapshot of Relay runtime counters.
type Stats struct {
	PacketsReceived        uint64
	PacketsForwarded       uint64
	PacketsDroppedDedup    uint64
	PacketsDroppedSelfEcho uint64
	ActiveInterfaces       int
}

// atomicStats holds the live counters; snapshot via .load().
type atomicStats struct {
	received        atomic.Uint64
	forwarded       atomic.Uint64
	droppedDedup    atomic.Uint64
	droppedSelfEcho atomic.Uint64
	activeIfaces    atomic.Int64
}

func (s *atomicStats) load() Stats {
	return Stats{
		PacketsReceived:        s.received.Load(),
		PacketsForwarded:       s.forwarded.Load(),
		PacketsDroppedDedup:    s.droppedDedup.Load(),
		PacketsDroppedSelfEcho: s.droppedSelfEcho.Load(),
		ActiveInterfaces:       int(s.activeIfaces.Load()),
	}
}

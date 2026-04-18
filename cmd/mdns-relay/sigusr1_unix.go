//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sonnt85/mdns/relay"
)

func usr1Stats(r *relay.Relay, logf func(string, ...any)) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	for range ch {
		s := r.Stats()
		logf("SIGUSR1 stats: recv=%d fwd=%d drop_dedup=%d drop_self=%d ifaces=%d",
			s.PacketsReceived, s.PacketsForwarded,
			s.PacketsDroppedDedup, s.PacketsDroppedSelfEcho,
			s.ActiveInterfaces)
	}
}

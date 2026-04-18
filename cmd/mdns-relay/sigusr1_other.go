//go:build !unix

package main

import "github.com/sonnt85/mdns/relay"

func usr1Stats(r *relay.Relay, logf func(string, ...any)) {
	// SIGUSR1 not available on Windows; no-op.
}

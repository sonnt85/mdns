// Package relay implements an mDNS reflector that forwards multicast DNS
// traffic between network interfaces. See README.md for usage.
package relay

import (
	"context"
	"errors"
	"log"
	"net"
	"time"
)

// Config configures a Relay.
type Config struct {
	// Include is the list of interface name patterns to listen on.
	// Glob supported: "eth0", "docker0", "br-*", "eth*".
	// Required.
	Include []string

	// Exclude is the list of patterns to ignore (applied after Include).
	Exclude []string

	// EnableIPv6 enables ff02::fb relaying in addition to IPv4.
	EnableIPv6 bool

	// WatchInterval controls how often net.Interfaces() is polled.
	// Zero disables watching (fixed set determined at startup).
	// Default: 5 * time.Second.
	WatchInterval time.Duration

	// DedupWindow is the TTL of the packet-hash dedup cache.
	// Default: 1 * time.Second.
	DedupWindow time.Duration

	// Logger is a printf-style log function. Nil uses log.Default().Printf.
	Logger func(format string, args ...any)

	// Watcher plugs a custom InterfaceWatcher (e.g. netlink).
	// Nil uses the built-in polling watcher.
	Watcher InterfaceWatcher
}

// InterfaceWatcher notifies the Relay when the active interface set changes.
type InterfaceWatcher interface {
	// Start begins watching. onChange is called at least once at startup
	// with the full current list, and again whenever the set changes.
	// Returns ctx.Err() on cancel.
	Start(ctx context.Context, onChange func([]net.Interface)) error
}

func (c *Config) applyDefaults() error {
	if len(c.Include) == 0 {
		return errors.New("relay: Config.Include is required")
	}
	if c.WatchInterval < 0 {
		return errors.New("relay: WatchInterval must be >= 0")
	}
	if c.DedupWindow < 0 {
		return errors.New("relay: DedupWindow must be >= 0")
	}
	if c.WatchInterval == 0 {
		c.WatchInterval = 5 * time.Second
	}
	if c.DedupWindow == 0 {
		c.DedupWindow = time.Second
	}
	if c.Logger == nil {
		c.Logger = log.Default().Printf
	}
	return nil
}

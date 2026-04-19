// relay/relay.go
package relay

import (
	"context"
	"errors"
	"net"
	"sync"
	"syscall"
)

// Relay reflects mDNS packets between network interfaces.
// See package doc for usage.
type Relay struct {
	cfg   Config
	stats atomicStats

	mu        sync.Mutex
	listeners map[string]*ifaceListener // keyed by iface name
	localIPs  map[string]bool           // for self-echo guard

	dedup *dedup
}

// New creates a Relay. Does not start listening — call Start.
func New(cfg *Config) (*Relay, error) {
	if cfg == nil {
		return nil, errors.New("relay: Config is nil")
	}
	c := *cfg
	if err := c.applyDefaults(); err != nil {
		return nil, err
	}
	return &Relay{
		cfg:       c,
		listeners: make(map[string]*ifaceListener),
		localIPs:  make(map[string]bool),
		dedup:     newDedup(c.DedupWindow),
	}, nil
}

// Start begins listening and relaying. Blocks until ctx is cancelled or
// a fatal error. Returns ctx.Err() on clean shutdown.
func (r *Relay) Start(ctx context.Context) error {
	watcher := r.cfg.Watcher
	if watcher == nil {
		watcher = newPollingWatcher(r.cfg.WatchInterval)
	}

	var wg sync.WaitGroup
	defer func() {
		r.closeAll()
		wg.Wait()
	}()

	onChange := func(ifaces []net.Interface) {
		r.reconcile(ctx, ifaces, &wg)
	}

	return watcher.Start(ctx, onChange)
}

// Stats returns a snapshot of runtime counters.
func (r *Relay) Stats() Stats {
	s := r.stats.load()
	r.mu.Lock()
	s.ActiveInterfaces = len(r.listeners)
	r.mu.Unlock()
	return s
}

// reconcile opens listeners for matching ifaces and closes listeners for
// vanished/unmatched ifaces.
func (r *Relay) reconcile(ctx context.Context, ifaces []net.Interface, wg *sync.WaitGroup) {
	keep := map[string]bool{}
	var active []*net.Interface

	for i := range ifaces {
		iface := ifaces[i] // copy
		if !matchInterface(iface.Name, r.cfg.Include, r.cfg.Exclude) {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		keep[iface.Name] = true
		active = append(active, &iface)

		r.mu.Lock()
		_, already := r.listeners[iface.Name]
		r.mu.Unlock()
		if already {
			continue
		}

		l, err := openListener(&iface, r.cfg.EnableIPv6)
		if err != nil {
			r.cfg.Logger("relay: open %s: %v", iface.Name, err)
			continue
		}

		r.mu.Lock()
		r.listeners[iface.Name] = l
		r.stats.activeIfaces.Store(int64(len(r.listeners)))
		r.mu.Unlock()
		r.cfg.Logger("relay: listening on %s", iface.Name)

		wg.Add(1)
		go r.recvLoop(ctx, l, wg)
	}

	// close listeners for ifaces that no longer match
	r.mu.Lock()
	var toClose []*ifaceListener
	for name, l := range r.listeners {
		if !keep[name] {
			toClose = append(toClose, l)
			delete(r.listeners, name)
			r.cfg.Logger("relay: stopped listening on %s", name)
		}
	}
	r.stats.activeIfaces.Store(int64(len(r.listeners)))
	r.localIPs = collectLocalIPs(active)
	r.mu.Unlock()

	for _, l := range toClose {
		l.close()
	}
}

// recvLoop spawns per-connection goroutines that read packets and fan them out.
func (r *Relay) recvLoop(ctx context.Context, l *ifaceListener, wg *sync.WaitGroup) {
	defer wg.Done()

	wg.Add(1)
	go r.recvConn(ctx, l, l.v4, mdnsIPv4, wg)

	if l.v6 != nil {
		wg.Add(1)
		go r.recvConn(ctx, l, l.v6, mdnsIPv6, wg)
	}
}

// recvConn reads packets from conn and fans them out. dst is the multicast
// address to send on (for fan-out writes).
func (r *Relay) recvConn(ctx context.Context, srcL *ifaceListener, conn *net.UDPConn, dst *net.UDPAddr, wg *sync.WaitGroup) {
	defer wg.Done()
	isV6 := dst == mdnsIPv6
	buf := make([]byte, 65536)
	for {
		if ctx.Err() != nil {
			return
		}
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		r.stats.received.Add(1)
		r.handleFamily(srcL, buf[:n], from, isV6)
	}
}

// handleFamily runs the three loop-prevention layers then fans out to the
// appropriate address family (IPv4 or IPv6).
func (r *Relay) handleFamily(srcL *ifaceListener, pkt []byte, from *net.UDPAddr, isV6 bool) {
	r.mu.Lock()
	if isSelfEcho(from, r.localIPs) {
		r.mu.Unlock()
		r.stats.droppedSelfEcho.Add(1)
		return
	}
	r.mu.Unlock()

	if r.dedup.seen(pkt) {
		r.stats.droppedDedup.Add(1)
		return
	}

	r.mu.Lock()
	targets := make([]*ifaceListener, 0, len(r.listeners))
	for name, l := range r.listeners {
		if name == srcL.iface.Name {
			continue
		}
		targets = append(targets, l)
	}
	r.mu.Unlock()

	dst := mdnsIPv4
	if isV6 {
		dst = mdnsIPv6
	}
	for _, l := range targets {
		conn := l.v4
		if isV6 {
			conn = l.v6
			if conn == nil {
				continue // target iface has no v6 listener
			}
		}
		if _, err := conn.WriteToUDP(pkt, dst); err != nil {
			r.cfg.Logger("relay: write to %s (v6=%v): %v", l.iface.Name, isV6, err)
			if isDeviceGone(err) {
				r.removeListener(l.iface.Name)
			}
			continue
		}
		r.stats.forwarded.Add(1)
		if r.cfg.Verbose {
			r.cfg.Logger("relay: fwd %s->%s %d bytes (v6=%v)", srcL.iface.Name, l.iface.Name, len(pkt), isV6)
		}
	}
}

// isDeviceGone reports whether err indicates the underlying network
// interface has vanished (e.g. docker network removed). When true, the
// listener for that interface should be torn down eagerly instead of
// waiting for the next watcher poll.
func isDeviceGone(err error) bool {
	return errors.Is(err, syscall.ENODEV) ||
		errors.Is(err, syscall.ENETDOWN) ||
		errors.Is(err, syscall.ENETUNREACH)
}

// removeListener closes and removes the listener for ifname. Safe to call
// when the listener does not exist (no-op).
func (r *Relay) removeListener(ifname string) {
	r.mu.Lock()
	l, ok := r.listeners[ifname]
	if ok {
		delete(r.listeners, ifname)
		r.stats.activeIfaces.Store(int64(len(r.listeners)))
	}
	r.mu.Unlock()
	if ok {
		l.close()
		r.cfg.Logger("relay: stopped listening on %s (device gone)", ifname)
	}
}

func (r *Relay) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, l := range r.listeners {
		l.close()
		delete(r.listeners, name)
	}
	r.stats.activeIfaces.Store(0)
}

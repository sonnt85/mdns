//go1:build integration

package client

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sonnt85/mdns/server"
	"github.com/sonnt85/mdns/service"
)

// TestClient_ServerLookup is an end-to-end integration test: it
// starts a real server and verifies client.Query can discover it
// over multicast. It is flaky in sandboxed environments where
// multicast loopback is restricted.
func TestClient_ServerLookup(t *testing.T) {
	zone, err := service.New(
		"hostname",
		"_foobar._tcp",
		"local.",
		"testhost.",
		80,
		[]net.IP{net.IP([]byte{192, 168, 0, 42}), net.ParseIP("2620:0:1000:1900:b0c2:d0b2:c411:18bc")},
		[]string{"Local web server"})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	serv, err := server.New(&server.Config{Zone: zone})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	defer func() { _ = serv.Shutdown() }()

	timeout := time.Second * 300
	serviceName := "_services._dns-sd._udp"
	entries := make(chan *ServiceEntry, 5)
	var found int32 = 0
	var errMsg atomic.Value
	go func() {
		// Note: do not call t.Logf/t.Fatalf here. This goroutine may
		// race with the parent test returning (e.g. after t.Fatalf on
		// Query or the assertions below). Communicate results through
		// errMsg (atomic.Value) and the `found` atomic counter instead.
		select {
		case e := <-entries:
			if serviceName != "_services._dns-sd._udp" {
				if e.Name != "hostname._foobar._tcp.local." {
					errMsg.Store("bad name: " + e.Name)
					return
				}
				if e.Port != 80 {
					errMsg.Store("bad port")
					return
				}
				if e.Info != "Local web server" {
					errMsg.Store("bad info: " + e.Info)
					return
				}
			}
			atomic.StoreInt32(&found, 1)

		case <-time.After(timeout):
			errMsg.Store("timeout waiting for entry")
		}
	}()

	params := &QueryParam{
		Service: serviceName,
		Domain:  "local",
		Timeout: timeout,
		Entries: entries,
	}
	if err := Query(params); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if msg := errMsg.Load(); msg != nil {
		t.Fatalf("goroutine error: %v", msg)
	}
	if atomic.LoadInt32(&found) == 0 {
		t.Fatalf("record not found")
	}
}

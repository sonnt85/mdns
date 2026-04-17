package mdns

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestServer_StartStop(t *testing.T) {
	s := makeService(t)
	serv, err := NewServer(&Config{Zone: s})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer serv.Shutdown()
}

func TestServer_Lookup(t *testing.T) {
	serv, err := NewServer(&Config{Zone: makeServiceWithServiceName(t, "_foobar._tcp")})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer serv.Shutdown()
	timeout := time.Second * 300
	serviceName := "_services._dns-sd._udp"
	entries := make(chan *ServiceEntry, 5)
	var found int32 = 0
	var errMsg atomic.Value
	go func() {
		// Note: do not call t.Logf/t.Fatalf here. This goroutine may race
		// with the parent test returning (e.g. after t.Fatalf on Query or
		// the assertions below). Communicate results through errMsg
		// (atomic.Value) and the `found` atomic counter instead.
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
	err = Query(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if msg := errMsg.Load(); msg != nil {
		t.Fatalf("goroutine error: %v", msg)
	}
	if atomic.LoadInt32(&found) == 0 {
		t.Fatalf("record not found")
	}
}

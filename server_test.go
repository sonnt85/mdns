package mdns

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/sonnt85/gosutils/ppjson"
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
	go func() {
		select {
		case e := <-entries:
			// fmt.Println(e)
			ppjson.Println(e)
			if serviceName != "_services._dns-sd._udp" {
				if e.Name != "hostname._foobar._tcp.local." {
					t.Fatalf("bad: %v", e)
				}
				if e.Port != 80 {
					t.Fatalf("bad: %v", e)
				}
				if e.Info != "Local web server" {
					t.Fatalf("bad: %v", e)
				}
			}
			atomic.StoreInt32(&found, 1)

		case <-time.After(timeout):
			t.Fatalf("timeout")
		}
	}()

	params := &QueryParam{
		// Service: "_foobar._tcp",
		Service: serviceName,
		Domain:  "local",
		Timeout: timeout,
		Entries: entries,
	}
	err = Query(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if atomic.LoadInt32(&found) == 0 {
		t.Fatalf("record not found")
	}
}

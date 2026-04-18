package server

import (
	"net"
	"testing"

	"github.com/sonnt85/mdns/service"
)

func makeTestZone(t *testing.T, name string) service.Zone {
	t.Helper()
	s, err := service.New(
		"hostname",
		name,
		"local.",
		"testhost.",
		80,
		[]net.IP{net.IP([]byte{192, 168, 0, 42}), net.ParseIP("2620:0:1000:1900:b0c2:d0b2:c411:18bc")},
		[]string{"Local web server"})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return s
}

func TestServer_StartStop(t *testing.T) {
	serv, err := New(&Config{Zone: makeTestZone(t, "_http._tcp")})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer func() { _ = serv.Shutdown() }()
}

func TestNew_RejectsMissingConfig(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNew_RejectsMissingZone(t *testing.T) {
	if _, err := New(&Config{}); err == nil {
		t.Fatal("expected error for missing Zone")
	}
}

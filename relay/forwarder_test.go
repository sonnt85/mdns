package relay

import (
	"net"
	"testing"
)

func TestForwarderSelfIPDrop(t *testing.T) {
	myIPs := map[string]bool{"192.168.1.10": true}
	src := &net.UDPAddr{IP: net.ParseIP("192.168.1.10"), Port: 5353}
	if !isSelfEcho(src, myIPs) {
		t.Error("should detect self-IP as echo")
	}

	src2 := &net.UDPAddr{IP: net.ParseIP("192.168.1.20"), Port: 5353}
	if isSelfEcho(src2, myIPs) {
		t.Error("other IP should not be self-echo")
	}
}

func TestCollectLocalIPs(t *testing.T) {
	// Use loopback — guaranteed present on all platforms.
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		// some platforms call it "lo0"
		lo, err = net.InterfaceByName("lo0")
	}
	if err != nil {
		t.Skip("no loopback interface to test with")
	}
	ips := collectLocalIPs([]*net.Interface{lo})
	if len(ips) == 0 {
		t.Errorf("expected at least one loopback IP, got %v", ips)
	}
}

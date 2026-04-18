// relay/listener.go
package relay

import (
	"fmt"
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

var (
	mdnsIPv4 = &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: 5353}
	mdnsIPv6 = &net.UDPAddr{IP: net.ParseIP("ff02::fb"), Port: 5353}
)

// ifaceListener wraps the IPv4 (and optional IPv6) multicast listeners for
// a single network interface.
type ifaceListener struct {
	iface *net.Interface
	v4    *net.UDPConn
	v6    *net.UDPConn
}

// openListener joins the mDNS multicast groups on iface.
// If enableV6 is true, also opens an IPv6 listener.
func openListener(iface *net.Interface, enableV6 bool) (*ifaceListener, error) {
	v4, err := net.ListenMulticastUDP("udp4", iface, mdnsIPv4)
	if err != nil {
		return nil, fmt.Errorf("open v4 on %s: %w", iface.Name, err)
	}
	// Ensure outbound multicast uses this interface.
	if err := ipv4.NewPacketConn(v4).SetMulticastInterface(iface); err != nil {
		v4.Close()
		return nil, fmt.Errorf("set v4 mcast iface on %s: %w", iface.Name, err)
	}

	l := &ifaceListener{iface: iface, v4: v4}

	if enableV6 {
		v6, err := net.ListenMulticastUDP("udp6", iface, mdnsIPv6)
		if err != nil {
			// IPv6 is optional per-iface; log via caller, don't fail hard
			return l, nil
		}
		if err := ipv6.NewPacketConn(v6).SetMulticastInterface(iface); err != nil {
			v6.Close()
			return l, nil
		}
		l.v6 = v6
	}
	return l, nil
}

func (l *ifaceListener) close() {
	if l.v4 != nil {
		l.v4.Close()
	}
	if l.v6 != nil {
		l.v6.Close()
	}
}

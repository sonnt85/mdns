// Package server implements the mDNS multicast responder. A Server
// listens for mDNS queries on the local network and answers those
// matching the Zone provided in Config.
package server

import (
	"net"

	"github.com/sonnt85/mdns/service"
)

// DefaultPort is the IANA-assigned mDNS multicast port (RFC 6762).
const DefaultPort = 5353

// Default multicast group addresses (RFC 6762).
var (
	DefaultIPv4Group = net.IPv4(224, 0, 0, 251)
	DefaultIPv6Group = net.ParseIP("ff02::fb")
)

// Config is used to configure the mDNS server.
type Config struct {
	// Zone must be provided to support responding to queries.
	Zone service.Zone

	// Iface binds the multicast listener to the given interface. Nil
	// uses the system default multicast interface.
	Iface *net.Interface

	// Port overrides DefaultPort. Zero means DefaultPort.
	Port int

	// UseIPv6 additionally opens an IPv6 multicast listener.
	UseIPv6 bool

	// ForceUnicast forces every answer to be sent back over unicast
	// regardless of RFC 6762's multicast preference.
	ForceUnicast bool

	// MulticastIPv4 overrides DefaultIPv4Group. Zero uses the default.
	MulticastIPv4 net.IP

	// MulticastIPv6 overrides DefaultIPv6Group. Zero uses the default.
	MulticastIPv6 net.IP

	// LogEmptyResponses indicates the server should print an
	// informative message when there is an mDNS query for which the
	// server has no response.
	LogEmptyResponses bool
}

func (c *Config) port() int {
	if c.Port != 0 {
		return c.Port
	}
	return DefaultPort
}

func (c *Config) ipv4Addr() *net.UDPAddr {
	ip := c.MulticastIPv4
	if len(ip) == 0 {
		ip = DefaultIPv4Group
	}
	return &net.UDPAddr{IP: ip, Port: c.port()}
}

func (c *Config) ipv6Addr() *net.UDPAddr {
	ip := c.MulticastIPv6
	if len(ip) == 0 {
		ip = DefaultIPv6Group
	}
	return &net.UDPAddr{IP: ip, Port: c.port()}
}

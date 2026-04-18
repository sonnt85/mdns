// Package client implements mDNS service discovery. Query sends a
// question onto the local multicast network and streams discovered
// service entries back over a channel.
package client

import (
	"net"
	"time"
)

// DefaultPort is the IANA-assigned mDNS multicast port (RFC 6762).
const DefaultPort = 5353

// Default multicast group addresses (RFC 6762).
var (
	DefaultIPv4Group = net.IPv4(224, 0, 0, 251)
	DefaultIPv6Group = net.ParseIP("ff02::fb")
)

// QueryParam is used to customize how a Lookup is performed.
type QueryParam struct {
	Service             string               // Service to lookup
	Domain              string               // Lookup domain, default "local"
	Timeout             time.Duration        // Lookup timeout, default 1 second
	Interface           *net.Interface       // Multicast interface to use
	Entries             chan<- *ServiceEntry // Entries Channel
	WantUnicastResponse bool                 // Unicast response desired, as per 5.4 in RFC

	// UseIPv6 additionally opens an IPv6 multicast listener.
	UseIPv6 bool

	// Port overrides DefaultPort. Zero uses DefaultPort.
	Port int

	// MulticastIPv4 overrides DefaultIPv4Group. Zero uses the default.
	MulticastIPv4 net.IP

	// MulticastIPv6 overrides DefaultIPv6Group. Zero uses the default.
	MulticastIPv6 net.IP
}

// DefaultParams returns a QueryParam populated with sensible defaults
// for service.
func DefaultParams(service string) *QueryParam {
	return &QueryParam{
		Service:             service,
		Domain:              "local",
		Timeout:             time.Second,
		Entries:             make(chan *ServiceEntry),
		WantUnicastResponse: false, // TODO(reddaly): Change this default.
	}
}

// Query looks up a given service, in a domain, waiting at most for a
// timeout before finishing the query. The results are streamed to a
// channel. Sends will not block, so callers should either read or
// buffer.
func Query(params *QueryParam) error {
	if params.Domain == "" {
		params.Domain = "local"
	}
	if params.Timeout == 0 {
		params.Timeout = time.Second
	}

	c, err := newClient(params)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if params.Interface != nil {
		if err := c.setInterface(params.Interface); err != nil {
			return err
		}
	}
	return c.query(params)
}

// Lookup is the same as Query, however it uses all the default
// parameters.
func Lookup(service string, entries chan<- *ServiceEntry) error {
	params := DefaultParams(service)
	params.Entries = entries
	return Query(params)
}

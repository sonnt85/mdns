package client

import (
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// client is the internal UDP plumbing. Callers use Query / Lookup.
type client struct {
	ipv4UnicastConn *net.UDPConn
	ipv6UnicastConn *net.UDPConn

	ipv4MulticastConn *net.UDPConn
	ipv6MulticastConn *net.UDPConn

	ipv4Addr *net.UDPAddr
	ipv6Addr *net.UDPAddr

	closed   int32
	closedCh chan struct{} // TODO(reddaly): This doesn't appear to be used.
}

// newClient creates an mDNS client bound to ephemeral ports, joining
// the multicast groups derived from params.
func newClient(params *QueryParam) (*client, error) {
	// TODO(reddaly): At least attempt to bind to the port required in the spec.
	port := params.Port
	if port == 0 {
		port = DefaultPort
	}
	v4ip := params.MulticastIPv4
	if len(v4ip) == 0 {
		v4ip = DefaultIPv4Group
	}
	v6ip := params.MulticastIPv6
	if len(v6ip) == 0 {
		v6ip = DefaultIPv6Group
	}
	ipv4Addr := &net.UDPAddr{IP: v4ip, Port: port}
	ipv6Addr := &net.UDPAddr{IP: v6ip, Port: port}

	var err error
	uconn4, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		log.Printf("[ERR] mdns: Failed to bind to udp4 port: %v", err)
	}
	var mconn6, uconn6 *net.UDPConn
	if params.UseIPv6 {
		if uconn6, err = net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6zero, Port: 0}); err != nil {
			log.Printf("[ERR] mdns: Failed to bind to udp6 port: %v", err)
		}
		if mconn6, err = net.ListenMulticastUDP("udp6", nil, ipv6Addr); err != nil {
			if len(ipv6Addr.IP) != 0 {
				log.Printf("mdns: Failed to bind to udp6 port: %v", err)
			}
		}
	}

	if uconn4 == nil && uconn6 == nil {
		return nil, fmt.Errorf("failed to bind to any unicast udp port")
	}

	mconn4, err := net.ListenMulticastUDP("udp4", nil, ipv4Addr)
	if err != nil {
		if len(ipv4Addr.IP) != 0 {
			log.Printf("mdns: Failed to bind to udp4 port: %v", err)
		}
	}

	if mconn4 == nil && mconn6 == nil {
		return nil, fmt.Errorf("failed to bind to any multicast udp port")
	}

	return &client{
		ipv4MulticastConn: mconn4,
		ipv6MulticastConn: mconn6,
		ipv4UnicastConn:   uconn4,
		ipv6UnicastConn:   uconn6,
		ipv4Addr:          ipv4Addr,
		ipv6Addr:          ipv6Addr,
		closedCh:          make(chan struct{}),
	}, nil
}

// Close releases any sockets held by the client.
func (c *client) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	close(c.closedCh)

	if c.ipv4UnicastConn != nil {
		_ = c.ipv4UnicastConn.Close()
	}
	if c.ipv6UnicastConn != nil {
		_ = c.ipv6UnicastConn.Close()
	}
	if c.ipv4MulticastConn != nil {
		_ = c.ipv4MulticastConn.Close()
	}
	if c.ipv6MulticastConn != nil {
		_ = c.ipv6MulticastConn.Close()
	}

	return nil
}

// setInterface pins all open connections to iface.
func (c *client) setInterface(iface *net.Interface) error {
	if c.ipv4MulticastConn == nil && c.ipv6MulticastConn == nil {
		return fmt.Errorf("can not set any interface  iptype [ip4, ip6]")
	}

	if c.ipv4UnicastConn != nil {
		p := ipv4.NewPacketConn(c.ipv4UnicastConn)
		if err := p.SetMulticastInterface(iface); err != nil {
			return err
		}
	}

	if c.ipv6UnicastConn != nil {
		p2 := ipv6.NewPacketConn(c.ipv6UnicastConn)
		if err := p2.SetMulticastInterface(iface); err != nil {
			return err
		}
	}

	if c.ipv4MulticastConn != nil {
		p := ipv4.NewPacketConn(c.ipv4MulticastConn)
		if err := p.SetMulticastInterface(iface); err != nil {
			return err
		}
	}

	if c.ipv6MulticastConn != nil {
		p2 := ipv6.NewPacketConn(c.ipv6MulticastConn)
		if err := p2.SetMulticastInterface(iface); err != nil {
			return err
		}
	}
	return nil
}

// regexpCache stores compiled regex patterns to avoid recompilation on every call.
var regexpCache sync.Map

func regexpMatch(parterm, strcompare string) bool {
	if cached, ok := regexpCache.Load(parterm); ok {
		return cached.(*regexp.Regexp).MatchString(strcompare)
	}
	rx, err := regexp.Compile(parterm)
	if err != nil {
		return false
	}
	regexpCache.Store(parterm, rx)
	return rx.MatchString(strcompare)
}

// query performs the actual lookup and streams results to params.Entries.
func (c *client) query(params *QueryParam) error {
	// Create the service name
	rexpprefix := "~"
	serviceAddr := fmt.Sprintf("%s.%s.", strings.Trim(params.Service, "."), strings.Trim(params.Domain, "."))
	if strings.HasPrefix(serviceAddr, rexpprefix) {
		_, repartem, ok := strings.Cut(serviceAddr, rexpprefix)
		if ok {
			rexpprefix = ""
			serviceAddr = repartem
		}
	}
	sdServiceAddr := fmt.Sprintf("_services._dns-sd._udp.%s.", strings.Trim(params.Domain, "."))

	// Start listening for response packets
	msgCh := make(chan *dns.Msg, 32)
	var recvWg sync.WaitGroup
	startRecv := func(conn *net.UDPConn) {
		recvWg.Add(1)
		go func() {
			defer recvWg.Done()
			c.recv(conn, msgCh)
		}()
	}
	startRecv(c.ipv4UnicastConn)
	startRecv(c.ipv6UnicastConn)
	startRecv(c.ipv4MulticastConn)
	startRecv(c.ipv6MulticastConn)

	// Ensure recv goroutines are fully stopped and msgCh is closed on return.
	defer func() {
		_ = c.Close()
		recvWg.Wait()
		close(msgCh)
	}()

	// Send the query
	m := new(dns.Msg)
	if len(rexpprefix) == 0 {
		m.SetQuestion(sdServiceAddr, dns.TypePTR)
	} else {
		m.SetQuestion(serviceAddr, dns.TypePTR)
	}
	// RFC 6762, section 18.12.  Repurposing of Top Bit of qclass in Question
	// Section
	//
	// In the Question Section of a Multicast DNS query, the top bit of the qclass
	// field is used to indicate that unicast responses are preferred for this
	// particular question.  (See Section 5.4.)
	if params.WantUnicastResponse {
		m.Question[0].Qclass |= 1 << 15
	}
	m.RecursionDesired = false
	if err := c.sendQuery(m); err != nil {
		fmt.Println("query", err)
		return err
	}

	// Map the in-progress responses
	inprogress := make(map[string]*ServiceEntry)

	finishTimer := time.NewTimer(params.Timeout)
	startTime := time.Now()
	for {
		select {
		case resp := <-msgCh:
			var inp *ServiceEntry
			for _, answer := range append(resp.Answer, resp.Extra...) {
				// TODO(reddaly): Check that response corresponds to serviceAddr?
				switch rr := answer.(type) {
				case *dns.PTR:
					inp = ensureName(inprogress, rr.Ptr)

				case *dns.SRV:
					if rr.Target != rr.Hdr.Name {
						alias(inprogress, rr.Hdr.Name, rr.Target)
					}

					inp = ensureName(inprogress, rr.Hdr.Name)

					if !strings.HasSuffix(inp.Name, serviceAddr) && sdServiceAddr != serviceAddr && !regexpMatch(serviceAddr, inp.Name) {
						break
					}
					inp.Host = rr.Target
					inp.Port = int(rr.Port)

				case *dns.TXT:
					inp = ensureName(inprogress, rr.Hdr.Name)
					if !strings.HasSuffix(inp.Name, serviceAddr) && sdServiceAddr != serviceAddr && !regexpMatch(serviceAddr, inp.Name) {
						break
					}
					inp.Info = strings.Join(rr.Txt, "|")
					inp.InfoFields = rr.Txt
					inp.hasTXT = true

				case *dns.A:
					inp = ensureName(inprogress, rr.Hdr.Name)
					if !strings.HasSuffix(inp.Name, serviceAddr) && sdServiceAddr != serviceAddr && !regexpMatch(serviceAddr, inp.Name) {
						break
					}
					inp.Addr = rr.A
					inp.AddrV4 = rr.A

				case *dns.AAAA:
					inp = ensureName(inprogress, rr.Hdr.Name)
					if !strings.HasSuffix(rr.Hdr.Name, serviceAddr) && sdServiceAddr != serviceAddr && !regexpMatch(serviceAddr, rr.Hdr.Name) {
						break
					}
					inp.Addr = rr.AAAA
					inp.AddrV6 = rr.AAAA
				}
			}

			if inp == nil {
				continue
			}

			// Check if this entry is complete
			if inp.complete() {
				if inp.sent {
					continue
				}
				inp.sent = true
				if startTime.Add(params.Timeout * 4).After(time.Now()) {
					finishTimer.Reset(params.Timeout)
				}
				select {
				case params.Entries <- inp:
				default:
					log.Printf("[WARN] mdns: dropped service entry %q (Entries channel full)", inp.Name)
				}
			} else {
				// Fire off a node specific query
				m := new(dns.Msg)
				m.SetQuestion(inp.Name, dns.TypePTR)
				m.RecursionDesired = false
				if err := c.sendQuery(m); err != nil {
					log.Printf("[ERR] mdns: Failed to query instance %s: %v", inp.Name, err)
				}
			}
		case <-finishTimer.C:
			return nil
		}
	}
}

// sendQuery is used to multicast a query out.
func (c *client) sendQuery(q *dns.Msg) error {
	errs := ""

	buf, err := q.Pack()
	cnterr := 0
	cntsend := 0
	if err != nil {
		return err
	}
	if c.ipv4UnicastConn != nil {
		cntsend++
		_, err = c.ipv4UnicastConn.WriteToUDP(buf, c.ipv4Addr)
		if err != nil {
			errs = errs + "\n" + err.Error()
			cnterr++
		}
	}
	if c.ipv6UnicastConn != nil {
		cntsend++
		_, err = c.ipv6UnicastConn.WriteToUDP(buf, c.ipv6Addr)
		if err != nil {
			errs = errs + "\n" + err.Error()
			cnterr++
		}
	}
	// Return error only if ALL sends failed
	if cntsend > 0 && cnterr == cntsend {
		return errors.New("mdns: failed to send query on all interfaces\n" + errs)
	}
	return nil
}

// recv is used to receive until we get a shutdown.
func (c *client) recv(l *net.UDPConn, msgCh chan *dns.Msg) {
	if l == nil {
		return
	}
	buf := make([]byte, 65536)
	for atomic.LoadInt32(&c.closed) == 0 {
		n, err := l.Read(buf)

		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		if err != nil {
			log.Printf("[ERR] mdns: Failed to read packet: %v", err)
			continue
		}
		msg := new(dns.Msg)
		if err := msg.Unpack(buf[:n]); err != nil {
			log.Printf("[ERR] mdns: Failed to unpack packet: %v", err)
			continue
		}
		select {
		case msgCh <- msg:
		case <-c.closedCh:
			return
		}
	}
}

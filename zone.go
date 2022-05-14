package mdns

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/miekg/dns"
)

const (
	// defaultTTL is the default TTL value in returned DNS records in seconds.
	defaultTTL = 120
)

// Zone is the interface used to integrate with the server and
// to serve records dynamically
type Zone interface {
	// Records returns DNS records in response to a DNS question.
	Records(q dns.Question) []dns.RR
}

// MDNSService is used to export a named service by implementing a Zone
type MDNSService struct {
	Instance string // Instance name (e.g. "hostService name")
	Service  string // Service name (e.g. "_http._tcp.")
	Domain   string // If blank, assumes "local"
	HostName string // Host machine DNS name (e.g. "mymachine.net.")
	Port     *int   // Service Port
	//	IPs      []net.IP    // IP addresses for the service's host
	IPs interface{} // IP addresses for the service's host
	TXT interface{} // Service TXT records

	serviceAddr  string // Fully qualified service address
	instanceAddr string // Fully qualified instance address
	enumAddr     string // _services._dns-sd._udp.<domain>
}

// validateFQDN returns an error if the passed string is not a fully qualified
// hdomain name (more specifically, a hostname).
func validateFQDN(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("FQDN must not be blank")
	}
	if s[len(s)-1] != '.' {
		return fmt.Errorf("FQDN must end in period: %s", s)
	}
	// TODO(reddaly): Perform full validation.

	return nil
}

// NewMDNSService returns a new instance of MDNSService.
//
// If domain, hostName, or ips is set to the zero value, then a default value
// will be inferred from the operating system.
//
// TODO(reddaly): This interface may need to change to account for "unique
// record" conflict rules of the mDNS protocol.  Upon startup, the server should
// check to ensure that the instance name does not conflict with other instance
// names, and, if required, select a new name.  There may also be conflicting
// hostName A/AAAA records.
// ipsI: net.IP,  *net.IP, []net.IP, *[]net.IP, string, *string, []string, *[]string => *[]net.IP
// portI: int, *int => *int
// txtI: string, []string, *[]string => *[]string
// txtI: string, []string, *[]string => *[]string
// domain: empty -> .local,
// hostName empty -> os.Hostname
func NewMDNSService(instance, service, domain, hostName string, portI interface{}, ipsI interface{}, txtI interface{}) (*MDNSService, error) {
	// Sanity check inputs
	var port *int
	//	var ips = new([]net.IP)
	var txt = new([]string)

	if instance == "" {
		return nil, fmt.Errorf("missing service instance name")
	}
	if service == "" {
		return nil, fmt.Errorf("missing service name")
	}

	switch v := portI.(type) {
	case int:
		port = &v
	case *int:
		port = v
	default:
	}

	if port == nil || *port == 0 {
		return nil, fmt.Errorf("missing service port")
	}

	// Set default domain
	if domain == "" {
		domain = "local."
	}
	if err := validateFQDN(domain); err != nil {
		return nil, fmt.Errorf("domain %q is not a fully-qualified domain name: %v", domain, err)
	}

	// Get host information if no host is specified.
	if hostName == "" {
		var err error
		hostName, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("could not determine host: %v", err)
		}
		hostName = fmt.Sprintf("%s.", hostName)
	}
	if err := validateFQDN(hostName); err != nil {
		return nil, fmt.Errorf("hostName %q is not a fully-qualified domain name: %v", hostName, err)
	}

	switch v := txtI.(type) {
	case string:
		txt = &[]string{v}
	case []string:
		txt = &v
	case *[]string:
		txt = v
	default:
	}
	return &MDNSService{
		Instance:     instance,
		Service:      service,
		Domain:       domain,
		HostName:     hostName,
		Port:         port,
		IPs:          ipsI,
		TXT:          txt,
		serviceAddr:  fmt.Sprintf("%s.%s.", trimDot(service), trimDot(domain)),
		instanceAddr: fmt.Sprintf("%s.%s.%s.", instance, trimDot(service), trimDot(domain)),
		enumAddr:     fmt.Sprintf("_services._dns-sd._udp.%s.", trimDot(domain)),
	}, nil
}

// trimDot is used to trim the dots from the start or end of a string
func trimDot(s string) string {
	return strings.Trim(s, ".")
}

// serviceRecords is called when the query matches the service name
func i2netIP(i interface{}) (ips []net.IP) {
	switch v := i.(type) {
	case net.IP:
		ips = []net.IP{v}
	case *net.IP:
		ips = []net.IP{*v}
	case []net.IP:
		ips = v
	case *[]net.IP:
		ips = *v
	case string:
		if ip := net.ParseIP(v); ip != nil && (ip.To4() != nil || ip.To16() != nil) {
			ips = []net.IP{ip}
		}
	case *string:
		if ip := net.ParseIP(*v); ip != nil {
			ips = []net.IP{ip}
		}
	case []string:
		for _, ipstr := range v {
			if ip := net.ParseIP(ipstr); ip != nil && (ip.To4() != nil || ip.To16() != nil) {
				ips = append(ips, ip)
			}
		}
	case *[]string:
		for _, ipstr := range *v {
			if ip := net.ParseIP(ipstr); ip != nil && (ip.To4() != nil || ip.To16() != nil) {
				ips = append(ips, ip)
			}
		}
	default:
	}
	return ips
}

func (m *MDNSService) getIPs() (ips []net.IP) {
	ips = i2netIP(m.IPs)
	return ips
}

func (m *MDNSService) gettxt() (txt []string) {
	switch v := m.TXT.(type) {
	case *[]string:
		txt = *v
	case *(*[]string):
		txt = **v
	case string:
		txt = []string{v}
	case *string:
		txt = []string{*v}
	default:
	}
	return txt
}

// Records returns DNS records in response to a DNS question.
func (m *MDNSService) Records(q dns.Question) []dns.RR {
	switch q.Name {
	case m.enumAddr:
		return m.serviceEnum(q)
	case m.serviceAddr:
		return m.serviceRecords(q)
	case m.instanceAddr:
		return m.instanceRecords(q)
	case m.HostName:
		if q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA {
			return m.instanceRecords(q)
		}
		fallthrough
	default:
		return nil
	}
}

func (m *MDNSService) serviceEnum(q dns.Question) []dns.RR {
	switch q.Qtype {
	case dns.TypeANY:
		fallthrough
	case dns.TypePTR:
		rr := &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    defaultTTL,
			},
			Ptr: m.serviceAddr,
		}
		return []dns.RR{rr}
	default:
		return nil
	}
}

// serviceRecords is called when the query matches the service name
func (m *MDNSService) serviceRecords(q dns.Question) []dns.RR {
	switch q.Qtype {
	case dns.TypeANY:
		fallthrough
	case dns.TypePTR:
		// Build a PTR response for the service
		rr := &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    defaultTTL,
			},
			Ptr: m.instanceAddr,
		}
		servRec := []dns.RR{rr}

		// Get the instance records
		instRecs := m.instanceRecords(dns.Question{
			Name:  m.instanceAddr,
			Qtype: dns.TypeANY,
		})

		// Return the service record with the instance records
		return append(servRec, instRecs...)
	default:
		return nil
	}
}

// serviceRecords is called when the query matches the instance name
func (m *MDNSService) instanceRecords(q dns.Question) []dns.RR {
	switch q.Qtype {
	case dns.TypeANY:
		// Get the SRV, which includes A and AAAA
		recs := m.instanceRecords(dns.Question{
			Name:  m.instanceAddr,
			Qtype: dns.TypeSRV,
		})

		// Add the TXT record
		recs = append(recs, m.instanceRecords(dns.Question{
			Name:  m.instanceAddr,
			Qtype: dns.TypeTXT,
		})...)
		return recs

	case dns.TypeA:
		var rr []dns.RR
		for _, ip := range m.getIPs() {
			if ip4 := ip.To4(); ip4 != nil {
				rr = append(rr, &dns.A{
					Hdr: dns.RR_Header{
						Name:   m.HostName,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    defaultTTL,
					},
					A: ip4,
				})
			}
		}
		return rr

	case dns.TypeAAAA:
		var rr []dns.RR
		for _, ip := range m.getIPs() {
			if ip.To4() != nil {
				// TODO(reddaly): IPv4 addresses could be encoded in IPv6 format and
				// putinto AAAA records, but the current logic puts ipv4-encodable
				// addresses into the A records exclusively.  Perhaps this should be
				// configurable?
				continue
			}

			if ip16 := ip.To16(); ip16 != nil {
				rr = append(rr, &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   m.HostName,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    defaultTTL,
					},
					AAAA: ip16,
				})
			}
		}
		return rr

	case dns.TypeSRV:
		// Create the SRV Record
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    defaultTTL,
			},
			Priority: 10,
			Weight:   1,
			Port:     uint16(*m.Port),
			Target:   m.HostName,
		}
		recs := []dns.RR{srv}

		// Add the A record
		recs = append(recs, m.instanceRecords(dns.Question{
			Name:  m.instanceAddr,
			Qtype: dns.TypeA,
		})...)

		// Add the AAAA record
		recs = append(recs, m.instanceRecords(dns.Question{
			Name:  m.instanceAddr,
			Qtype: dns.TypeAAAA,
		})...)
		return recs

	case dns.TypeTXT:
		txt := &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    defaultTTL,
			},
			Txt: m.gettxt(),
		}
		return []dns.RR{txt}
	}
	return nil
}

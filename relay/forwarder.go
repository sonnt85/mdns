package relay

import "net"

// isSelfEcho reports whether src came from one of our own interface IPs.
// Guards against a packet we just sent being read back and re-forwarded.
func isSelfEcho(src *net.UDPAddr, localIPs map[string]bool) bool {
	if src == nil {
		return false
	}
	return localIPs[src.IP.String()]
}

// collectLocalIPs returns the set of unicast IPs assigned to the given
// interfaces, as a set of string-form IPs for O(1) membership checks.
func collectLocalIPs(ifaces []*net.Interface) map[string]bool {
	set := make(map[string]bool, 8)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil {
				set[ip.String()] = true
			}
		}
	}
	return set
}

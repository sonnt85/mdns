package client

import "net"

// ServiceEntry is returned after we query for a service.
type ServiceEntry struct {
	Name       string
	Host       string
	AddrV4     net.IP
	AddrV6     net.IP
	Port       int
	Info       string
	InfoFields []string

	Addr net.IP // Deprecated: prefer AddrV4 or AddrV6.

	hasTXT bool
	sent   bool
}

// complete reports whether the entry has enough info to emit.
func (s *ServiceEntry) complete() bool {
	return (s.AddrV4 != nil || s.AddrV6 != nil || s.Addr != nil) && s.Port != 0 && s.hasTXT
}

// ensureName returns the in-progress entry for name, creating one
// on first sight.
func ensureName(inprogress map[string]*ServiceEntry, name string) *ServiceEntry {
	if inp, ok := inprogress[name]; ok {
		return inp
	}
	inp := &ServiceEntry{Name: name}
	inprogress[name] = inp
	return inp
}

// alias records that dst should resolve to the same in-progress
// entry as src.
func alias(inprogress map[string]*ServiceEntry, src, dst string) {
	inprogress[dst] = ensureName(inprogress, src)
}

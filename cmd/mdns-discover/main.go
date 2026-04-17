// Command mdns-discover is a lightweight mDNS service discovery tool.
//
// Usage:
//
//	mdns-discover [flags] [service]
//
// Examples:
//
//	mdns-discover                          # enumerate all services
//	mdns-discover _http._tcp               # find HTTP services
//	mdns-discover -scan                    # full network scan (2-phase)
//	mdns-discover -regex '_https?._tcp'    # regex match
//	mdns-discover -json _ipp._tcp          # output as JSON
//	mdns-discover -timeout 10s _ssh._tcp   # custom timeout
//	mdns-discover -iface eth0 _http._tcp   # bind to specific interface
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sonnt85/mdns"
)

// Set via -ldflags at build time
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

type result struct {
	Name    string `json:"name"`
	Host    string `json:"host,omitempty"`
	IPv4    string `json:"ipv4,omitempty"`
	IPv6    string `json:"ipv6,omitempty"`
	Port    int    `json:"port,omitempty"`
	Info    string `json:"info,omitempty"`
	Service string `json:"service,omitempty"`
}

var (
	timeout   *time.Duration
	domain    *string
	iface     *string
	useRegex  *bool
	jsonOut   *bool
	ipv6      *bool
	port      *int
	unicast   *bool
	scanMode   *bool
	showVer    *bool
)

func main() {
	timeout = flag.Duration("timeout", 3*time.Second, "discovery timeout per query")
	domain = flag.String("domain", "local", "mDNS domain")
	iface = flag.String("iface", "", "network interface to use")
	useRegex = flag.Bool("regex", false, "treat service as regex pattern")
	jsonOut = flag.Bool("json", false, "output as JSON")
	ipv6 = flag.Bool("6", false, "enable IPv6")
	port = flag.Int("port", 0, "custom mDNS port (default 5353)")
	unicast = flag.Bool("unicast", false, "prefer unicast responses")
	scanMode = flag.Bool("scan", false, "full network scan: enumerate all service types, then query each")
	showVer = flag.Bool("version", false, "show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [service]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Discover mDNS services on the local network.\n\n")
		fmt.Fprintf(os.Stderr, "If no service is given, enumerates all services via _services._dns-sd._udp.\n")
		fmt.Fprintf(os.Stderr, "Use -scan for a thorough 2-phase scan of all services.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s                             # quick enumerate\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -scan                       # full 2-phase scan\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s _http._tcp                  # find HTTP services\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -regex '_https?._tcp'       # regex match\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -json -timeout 10s _ipp._tcp\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("mdns-discover %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		fmt.Printf("Author: sonnt85 <thanhson.rf@gmail.com>\n")
		return
	}

	if *ipv6 {
		mdns.ConfigUseIpv6(true)
	}
	if *port > 0 {
		mdns.InitPort(*port)
	}

	if *scanMode {
		runFullScan()
		return
	}

	// Single query mode
	service := "_services._dns-sd._udp"
	if flag.NArg() > 0 {
		service = flag.Arg(0)
	}
	if *useRegex {
		service = "~" + service
	}

	results := queryService(service)
	outputResults(results)
}

// runFullScan does a 2-phase scan:
// Phase 1: enumerate all service types via _services._dns-sd._udp
// Phase 2: query each discovered service type for actual instances
func runFullScan() {
	if !*jsonOut {
		fmt.Fprintf(os.Stderr, "[phase 1] enumerating service types...\n")
	}

	// Phase 1: get all service types
	typeEntries := make(chan *mdns.ServiceEntry, 64)
	serviceTypes := make(map[string]bool)
	var mu sync.Mutex

	go func() {
		for e := range typeEntries {
			// Service type entries look like "_http._tcp.local."
			name := strings.TrimSuffix(e.Name, "."+trimDot(*domain)+".")
			mu.Lock()
			serviceTypes[name] = true
			mu.Unlock()
		}
	}()

	params := &mdns.QueryParam{
		Service:             "_services._dns-sd._udp",
		Domain:              *domain,
		Timeout:             *timeout,
		Interface:           getIface(),
		Entries:             typeEntries,
		WantUnicastResponse: *unicast,
	}
	_ = mdns.Query(params)
	close(typeEntries)

	// Even if no complete entries, the library's inprogress map caught PTR names.
	// But those aren't sent to Entries. We need a different approach for phase 1:
	// Use regex mode to catch everything, or parse raw responses.
	//
	// Workaround: also add common well-known service types if none found
	if len(serviceTypes) == 0 {
		// Try regex mode to catch any service
		regexEntries := make(chan *mdns.ServiceEntry, 64)
		go func() {
			for e := range regexEntries {
				// Extract service type from instance name: "host._http._tcp.local." → "_http._tcp"
				if svc := extractServiceType(e.Name); svc != "" {
					mu.Lock()
					serviceTypes[svc] = true
					mu.Unlock()
				}
			}
		}()
		regexParams := &mdns.QueryParam{
			Service:             "~.*",
			Domain:              *domain,
			Timeout:             *timeout,
			Interface:           getIface(),
			Entries:             regexEntries,
			WantUnicastResponse: *unicast,
		}
		_ = mdns.Query(regexParams)
		close(regexEntries)
	}

	if !*jsonOut {
		if len(serviceTypes) == 0 {
			fmt.Fprintf(os.Stderr, "[phase 1] no service types found\n")
			fmt.Fprintln(os.Stderr, "no services found")
			return
		}
		fmt.Fprintf(os.Stderr, "[phase 1] found %d service type(s):", len(serviceTypes))
		for svc := range serviceTypes {
			fmt.Fprintf(os.Stderr, " %s", svc)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "[phase 2] querying each service type...\n")
	}

	// Phase 2: query each service type
	var allResults []result
	for svc := range serviceTypes {
		entries := queryService(svc)
		for i := range entries {
			entries[i].Service = svc
		}
		allResults = append(allResults, entries...)
	}

	// Deduplicate by name
	seen := make(map[string]bool)
	var unique []result
	for _, r := range allResults {
		if !seen[r.Name] {
			seen[r.Name] = true
			unique = append(unique, r)
		}
	}

	outputResults(unique)
}

// extractServiceType extracts "_http._tcp" from "hostname._http._tcp.local."
func extractServiceType(name string) string {
	// Find pattern: _xxx._tcp or _xxx._udp
	parts := strings.Split(name, ".")
	for i := 0; i+1 < len(parts); i++ {
		if strings.HasPrefix(parts[i], "_") &&
			(parts[i+1] == "_tcp" || parts[i+1] == "_udp") {
			return parts[i] + "." + parts[i+1]
		}
	}
	return ""
}

func trimDot(s string) string {
	return strings.Trim(s, ".")
}

func getIface() *net.Interface {
	if *iface == "" {
		return nil
	}
	netIface, err := net.InterfaceByName(*iface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: interface %q: %v\n", *iface, err)
		os.Exit(1)
	}
	return netIface
}

func queryService(service string) []result {
	entries := make(chan *mdns.ServiceEntry, 64)
	var results []result
	var mu sync.Mutex

	done := make(chan struct{})
	go func() {
		for e := range entries {
			r := entryToResult(e)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			if !*jsonOut && !*scanMode {
				printEntry(r)
			}
		}
		close(done)
	}()

	params := &mdns.QueryParam{
		Service:             service,
		Domain:              *domain,
		Timeout:             *timeout,
		Interface:           getIface(),
		Entries:             entries,
		WantUnicastResponse: *unicast,
	}
	_ = mdns.Query(params)
	close(entries)
	<-done

	return results
}

func entryToResult(e *mdns.ServiceEntry) result {
	r := result{
		Name: e.Name,
		Host: e.Host,
		Port: e.Port,
		Info: e.Info,
	}
	if e.AddrV4 != nil {
		r.IPv4 = e.AddrV4.String()
	}
	if e.AddrV6 != nil {
		r.IPv6 = e.AddrV6.String()
	}
	return r
}

func outputResults(results []result) {
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		return
	}

	if *scanMode {
		for _, r := range results {
			printEntry(r)
		}
	}

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "no services found")
	} else {
		fmt.Fprintf(os.Stderr, "\n%d service(s) found\n", len(results))
	}
}

func printEntry(r result) {
	addr := r.IPv4
	if addr == "" {
		addr = r.IPv6
	}
	if addr == "" {
		addr = r.Host
	}
	if r.Port > 0 {
		fmt.Printf("%-45s %s:%d", r.Name, addr, r.Port)
	} else if addr != "" {
		fmt.Printf("%-45s %s", r.Name, addr)
	} else {
		fmt.Printf("%s", r.Name)
	}
	if r.Info != "" {
		fmt.Printf("  [%s]", r.Info)
	}
	if r.Service != "" {
		fmt.Printf("  (%s)", r.Service)
	}
	fmt.Println()
}

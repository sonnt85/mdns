// Command mdns-server publishes an mDNS service and responds to A/AAAA
// queries for the advertised hostname. Typical use: run inside a container
// so "ping <host>.local" from the LAN (through a mdns relay) resolves to
// the container's IP.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sonnt85/mdns"
)

var version = "dev"

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	var (
		host    string
		ips     string
		service string
		domain  string
		port    int
		txt     stringsFlag
		iface   string
		showVer bool
	)

	defHost, _ := os.Hostname()
	if defHost == "" {
		defHost = "mdns-server"
	}

	flag.StringVar(&host, "host", defHost, "hostname to advertise (without .local)")
	flag.StringVar(&ips, "ip", "auto", "comma-separated IPs, or 'auto' to detect non-loopback addrs")
	flag.StringVar(&service, "service", "_workstation._tcp", "service type")
	flag.StringVar(&domain, "domain", "local", "domain")
	flag.IntVar(&port, "port", 22, "service port")
	flag.Var(&txt, "txt", "TXT record (repeatable, e.g. -txt key=val)")
	flag.StringVar(&iface, "iface", "", "bind to specific interface (empty = all)")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.Parse()

	if showVer {
		fmt.Println(version)
		return
	}

	var ipList []net.IP
	var err error
	if ips == "" || ips == "auto" {
		ipList, err = autoDetectIPs(iface)
		if err != nil {
			log.Fatalf("auto-detect IPs: %v", err)
		}
		if len(ipList) == 0 {
			log.Fatal("no non-loopback IPs found; specify -ip or check -iface")
		}
	} else {
		for _, s := range strings.Split(ips, ",") {
			ip := net.ParseIP(strings.TrimSpace(s))
			if ip == nil {
				log.Fatalf("invalid IP: %q", s)
			}
			ipList = append(ipList, ip)
		}
	}

	hostFQDN := host + "." + domain + "."

	txtRecords := []string(txt)
	if len(txtRecords) == 0 {
		txtRecords = []string{"mdns-server=" + version}
	}

	svc, err := mdns.NewMDNSService(host, service, domain+".", hostFQDN, port, ipList, txtRecords)
	if err != nil {
		log.Fatalf("NewMDNSService: %v", err)
	}

	var bindIface *net.Interface
	if iface != "" {
		bindIface, err = net.InterfaceByName(iface)
		if err != nil {
			log.Fatalf("interface %s: %v", iface, err)
		}
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: svc, Iface: bindIface})
	if err != nil {
		log.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = server.Shutdown() }()

	log.Printf("mdns-server %s", version)
	log.Printf("  hostname:  %s", hostFQDN)
	log.Printf("  ips:       %v", ipList)
	log.Printf("  service:   %s.%s.", service, domain)
	log.Printf("  port:      %d", port)
	if iface != "" {
		log.Printf("  iface:     %s", iface)
	}
	log.Printf("ready — press Ctrl+C to stop")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Printf("shutting down...")
	time.Sleep(100 * time.Millisecond) // let goodbye packets go out
}

// autoDetectIPs returns all non-loopback, non-link-local IPs of the given
// interface (or all interfaces if ifname is empty).
func autoDetectIPs(ifname string) ([]net.IP, error) {
	var ifaces []net.Interface
	if ifname != "" {
		i, err := net.InterfaceByName(ifname)
		if err != nil {
			return nil, err
		}
		ifaces = []net.Interface{*i}
	} else {
		all, err := net.Interfaces()
		if err != nil {
			return nil, err
		}
		ifaces = all
	}

	var ips []net.IP
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			if ipnet, ok := a.(*net.IPNet); ok {
				ip = ipnet.IP
			}
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

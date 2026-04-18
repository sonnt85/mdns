# mdns

[![GoDoc](https://pkg.go.dev/badge/github.com/sonnt85/mdns.svg)](https://pkg.go.dev/github.com/sonnt85/mdns)
[![Go Report Card](https://goreportcard.com/badge/github.com/sonnt85/mdns)](https://goreportcard.com/report/github.com/sonnt85/mdns)

**The only Go mDNS library with regex-based service discovery.**

> Other mDNS libraries only support exact service name lookup. This library lets you use regex patterns to discover services — like `grep` for your local network.

Includes **`mdns-grep`** — a cross-platform CLI tool. Download from [Releases](https://github.com/sonnt85/mdns/releases).

```bash
mdns-grep                              # enumerate all services
mdns-grep _http._tcp                   # find HTTP services
mdns-grep -regex '_https?._tcp'        # regex: HTTP + HTTPS
mdns-grep -scan -timeout 10s           # full 2-phase network scan
mdns-grep -json _ipp._tcp              # JSON output
```

### Why this library?

| Feature | sonnt85/mdns | hashicorp/mdns | grandcat/zeroconf | brutella/dnssd |
|---------|:---:|:---:|:---:|:---:|
| **Regex service discovery** | **Yes** | No | No | No |
| **2-phase network scan** | **Yes** | No | No | No |
| **CLI tool (mdns-grep)** | **Yes** | No | No | Yes |
| **Configurable port/multicast addr** | **Yes** | No | No | No |
| Minimal dependencies | 5 | 5 | 8 | 12 |
| IPv4 + IPv6 | Yes | Yes | Yes | Yes |

### Download mdns-grep

Pre-built binaries for 14 platforms — no Go installation required:

| OS | Architecture | Download |
|----|-------------|----------|
| **Linux** | x86_64, ARM64, ARMv7, x86, MIPS, RISC-V | [Releases](https://github.com/sonnt85/mdns/releases) |
| **macOS** | Intel, Apple Silicon (M1/M2/M3/M4) | [Releases](https://github.com/sonnt85/mdns/releases) |
| **Windows** | x64, ARM64 | [Releases](https://github.com/sonnt85/mdns/releases) |
| **FreeBSD** | x64, ARM64, ARM | [Releases](https://github.com/sonnt85/mdns/releases) |

---

Simple mDNS client/server library in Go. mDNS (Multicast DNS) can be used
to discover services on the local network without an authoritative DNS
server. This enables peer-to-peer discovery. Many networks restrict the
use of multicasting, which prevents mDNS from functioning — notably,
multicast cannot be used in cloud or shared infrastructure environments.
However it works well in most office, home, or private networks.

### mdns-relay — cross-network mDNS reflector

A second CLI, `mdns-relay`, reflects mDNS packets between interfaces —
useful for LAN ↔ Docker container service discovery without
`--network=host`. See [`relay/README.md`](relay/README.md).

```bash
go install github.com/sonnt85/mdns/cmd/mdns-relay@latest
sudo mdns-relay -i eth0,docker0,br-*
```

---

## Package layout

This repository is organised as four focused subpackages. Import only
what you need — the root import path exposes no symbols.

| Import path | Purpose |
|---|---|
| `github.com/sonnt85/mdns/service` | `Service` + `Zone` interface. Describe what to advertise. |
| `github.com/sonnt85/mdns/server`  | mDNS multicast responder that answers queries from a `Zone`. |
| `github.com/sonnt85/mdns/client`  | `Query` / `Lookup` for discovering services on the LAN. |
| `github.com/sonnt85/mdns/relay`   | Stateless reflector that forwards mDNS between interfaces. |

## Installation

```bash
go get github.com/sonnt85/mdns/client       # discover services
go get github.com/sonnt85/mdns/server       # answer queries
go get github.com/sonnt85/mdns/service      # build the zone served by server
```

## Quick Start

### Publishing a Service

```go
import (
    "os"

    "github.com/sonnt85/mdns/server"
    "github.com/sonnt85/mdns/service"
)

host, _ := os.Hostname()
svc, _ := service.New(host, "_foobar._tcp", "", "", 8000, nil, []string{"My awesome service"})

srv, _ := server.New(&server.Config{Zone: svc})
defer srv.Shutdown()
```

### Discovering Services

```go
import "github.com/sonnt85/mdns/client"

entriesCh := make(chan *client.ServiceEntry, 8)
go func() {
    for entry := range entriesCh {
        fmt.Printf("Got new entry: %v\n", entry)
    }
}()

client.Lookup("_foobar._tcp", entriesCh)
close(entriesCh)
```

## API Reference

### `service` — records and Zone

```go
type Zone interface {
    Records(q dns.Question) []dns.RR
}

type Service struct {
    Instance string      // Instance name (e.g. "myhost")
    Service  string      // Service name (e.g. "_http._tcp.")
    Domain   string      // Domain, default "local"
    HostName string      // Host machine DNS name (e.g. "mymachine.net.")
    Port     *int        // Service port
    IPs      interface{} // net.IP, []net.IP, string, []string, ...
    TXT      interface{} // string, []string, *[]string
}

func New(instance, service, domain, hostName string,
    portI, ipsI, txtI interface{}) (*Service, error)
```

`Service` implements `Zone` and advertises PTR, SRV, A, AAAA, TXT
records. `service.New` accepts flexible input shapes:

| Parameter | Accepted types |
|---|---|
| `portI` | `int`, `*int` |
| `ipsI`  | `net.IP`, `*net.IP`, `[]net.IP`, `*[]net.IP`, `string`, `*string`, `[]string`, `*[]string` |
| `txtI`  | `string`, `[]string`, `*[]string` |

If `domain` is empty, it defaults to `"local."`. If `hostName` is
empty, it uses `os.Hostname()`.

### `server` — mDNS responder

```go
type Config struct {
    Zone              service.Zone
    Iface             *net.Interface // bind to a specific interface
    Port              int            // default 5353
    UseIPv6           bool
    ForceUnicast      bool
    MulticastIPv4     net.IP         // default 224.0.0.251
    MulticastIPv6     net.IP         // default ff02::fb
    LogEmptyResponses bool
}

func New(cfg *Config) (*Server, error)
func (s *Server) Shutdown() error
```

`server.New` starts background listeners and returns a running
server. `Shutdown` is safe to call multiple times.

Defaults are exported as `server.DefaultPort`,
`server.DefaultIPv4Group`, and `server.DefaultIPv6Group`.

### `client` — service discovery

```go
type ServiceEntry struct {
    Name       string
    Host       string
    AddrV4     net.IP
    AddrV6     net.IP
    Port       int
    Info       string   // TXT records joined by "|"
    InfoFields []string
}

type QueryParam struct {
    Service             string
    Domain              string                // default "local"
    Timeout             time.Duration         // default 1s
    Interface           *net.Interface
    Entries             chan<- *ServiceEntry
    WantUnicastResponse bool

    UseIPv6        bool
    Port           int    // default 5353
    MulticastIPv4  net.IP // default 224.0.0.251
    MulticastIPv6  net.IP // default ff02::fb
}

func DefaultParams(service string) *QueryParam
func Query(params *QueryParam) error
func Lookup(service string, entries chan<- *ServiceEntry) error
```

**Important:** `Entries` must be buffered or actively read. Sends
are non-blocking — results are dropped if the channel is full.

Defaults are exported as `client.DefaultPort`,
`client.DefaultIPv4Group`, and `client.DefaultIPv6Group`.

## Advanced Usage

### Custom Query with Timeout and Interface

```go
params := client.DefaultParams("_http._tcp")
params.Timeout = 5 * time.Second
params.Interface = iface // *net.Interface
params.Entries = make(chan *client.ServiceEntry, 16)

go func() {
    for entry := range params.Entries {
        fmt.Printf("%s at %s:%d\n", entry.Name, entry.AddrV4, entry.Port)
    }
}()

if err := client.Query(params); err != nil {
    log.Fatal(err)
}
close(params.Entries)
```

### Regex Service Discovery

Prefix the service name with `~` to switch from exact matching to
regex filtering. This changes the query strategy:

1. **Without `~`** (exact): sends a PTR query for the specific service
   name → only receives responses for that service.
2. **With `~`** (regex): sends a PTR query for
   `_services._dns-sd._udp.local.` → receives ALL advertised services
   on the network → filters responses using the regex pattern.

```go
// Discover all TCP services on the network
params := client.DefaultParams("~_.*._tcp")
params.Timeout = 5 * time.Second
params.Entries = make(chan *client.ServiceEntry, 32)

go func() {
    for entry := range params.Entries {
        fmt.Printf("Found: %s at %s:%d\n", entry.Name, entry.AddrV4, entry.Port)
    }
}()

client.Query(params)
close(params.Entries)
```

More regex examples:

```go
// Match only HTTP and HTTPS services
client.DefaultParams("~_https?._tcp")

// Match any service containing "printer"
client.DefaultParams("~.*printer.*")

// Match services on a specific host pattern
client.DefaultParams("~myhost.*._http._tcp")
```

**Note:** Regex patterns use Go's `regexp` syntax. The pattern is
matched against the full service instance name (e.g.
`myhost._http._tcp.local.`).

**Matching examples** — given these services advertised on the
network:

```
myprinter._ipp._tcp.local.
office-hp._ipp._tcp.local.
webserver._http._tcp.local.
api._https._tcp.local.
nas._smb._tcp.local.
camera1._rtsp._udp.local.
```

| Pattern | Matches |
|---------|---------|
| `"_http._tcp"` (exact) | `webserver._http._tcp.local.` only |
| `"~_https?._tcp"` | `webserver._http._tcp.local.`, `api._https._tcp.local.` |
| `"~_ipp._tcp"` | `myprinter._ipp._tcp.local.`, `office-hp._ipp._tcp.local.` |
| `"~.*printer.*"` | `myprinter._ipp._tcp.local.` |
| `"~_.*._tcp"` | All TCP services (5 matches, excludes `camera1._rtsp._udp.local.`) |
| `"~_.*._udp"` | `camera1._rtsp._udp.local.` only |
| `"~(office\|nas).*"` | `office-hp._ipp._tcp.local.`, `nas._smb._tcp.local.` |

### Custom Zone Implementation

```go
import (
    "github.com/miekg/dns"
    "github.com/sonnt85/mdns/server"
)

type myZone struct{}

func (z *myZone) Records(q dns.Question) []dns.RR {
    // Return custom DNS records based on the question.
    return nil
}

srv, _ := server.New(&server.Config{Zone: &myZone{}})
defer srv.Shutdown()
```

### Custom Port and Multicast Group

The mDNS port and multicast groups are fields on both
`server.Config` and `client.QueryParam`. Zero values fall back to
the RFC 6762 defaults (port 5353, `224.0.0.251`, `ff02::fb`).

```go
srv, _ := server.New(&server.Config{
    Zone:          myZone,
    Port:          15353,
    UseIPv6:       true,
    MulticastIPv4: net.ParseIP("239.255.250.250"),
})

params := client.DefaultParams("_foobar._tcp")
params.Port = 15353
params.UseIPv6 = true
params.MulticastIPv4 = net.ParseIP("239.255.250.250")
```

## Contributing

Contributions are welcome! Feel free to:

- Open an [issue](https://github.com/sonnt85/mdns/issues) for bug
  reports or feature requests.
- Submit a pull request — all PRs are reviewed.
- Star the repo if you find it useful.

If you use `mdns-grep` or this library in your project, I'd love to
hear about it.

## License

MIT License — see [LICENSE](LICENSE) for details. Free for personal
and commercial use.

## Author

**sonnt85** — [thanhson.rf@gmail.com](mailto:thanhson.rf@gmail.com)

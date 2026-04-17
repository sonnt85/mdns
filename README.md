# mdns

[![GoDoc](https://pkg.go.dev/badge/github.com/sonnt85/mdns.svg)](https://pkg.go.dev/github.com/sonnt85/mdns)

Simple mDNS client/server library in Golang. mDNS or Multicast DNS can be
used to discover services on the local network without the use of an authoritative
DNS server. This enables peer-to-peer discovery. It is important to note that many
networks restrict the use of multicasting, which prevents mDNS from functioning.
Notably, multicast cannot be used in any sort of cloud, or shared infrastructure
environment. However it works well in most office, home, or private infrastructure
environments.

## Installation

```bash
go get github.com/sonnt85/mdns
```

## Quick Start

### Publishing a Service

```go
host, _ := os.Hostname()
info := []string{"My awesome service"}
service, _ := mdns.NewMDNSService(host, "_foobar._tcp", "", "", 8000, nil, info)

// Create the mDNS server, defer shutdown
server, _ := mdns.NewServer(&mdns.Config{Zone: service})
defer server.Shutdown()
```

### Discovering Services

```go
// Make a channel for results and start listening
entriesCh := make(chan *mdns.ServiceEntry, 8)
go func() {
    for entry := range entriesCh {
        fmt.Printf("Got new entry: %v\n", entry)
    }
}()

// Start the lookup
mdns.Lookup("_foobar._tcp", entriesCh)
close(entriesCh)
```

## API Reference

### Types

#### Zone

```go
type Zone interface {
    Records(q dns.Question) []dns.RR
}
```

Interface for responding to DNS queries. Implement this to provide custom record serving logic. `MDNSService` is the built-in implementation.

#### MDNSService

```go
type MDNSService struct {
    Instance string      // Instance name (e.g. "myhost")
    Service  string      // Service name (e.g. "_http._tcp.")
    Domain   string      // Domain, default "local"
    HostName string      // Host machine DNS name (e.g. "mymachine.net.")
    Port     *int        // Service port
    IPs      interface{} // IP addresses: net.IP, []net.IP, *[]net.IP, string, []string
    TXT      interface{} // TXT records: string, []string, *[]string
}
```

Implements `Zone` to export a named service. Supports PTR, SRV, A, AAAA, and TXT record types.

#### ServiceEntry

```go
type ServiceEntry struct {
    Name       string   // Service instance name
    Host       string   // Host target from SRV record
    AddrV4     net.IP   // IPv4 address from A record
    AddrV6     net.IP   // IPv6 address from AAAA record
    Port       int      // Port from SRV record
    Info       string   // TXT records joined by "|"
    InfoFields []string // Individual TXT record strings
}
```

Returned by `Query`/`Lookup` when a service is discovered. An entry is considered complete when it has at least one IP address, a port, and TXT records.

#### QueryParam

```go
type QueryParam struct {
    Service             string               // Service to lookup (e.g. "_http._tcp")
    Domain              string               // Lookup domain, default "local"
    Timeout             time.Duration        // Lookup timeout, default 1 second
    Interface           *net.Interface       // Multicast interface to use
    Entries             chan<- *ServiceEntry  // Channel to receive results
    WantUnicastResponse bool                 // Prefer unicast responses (RFC 6762 §5.4)
}
```

Configures how a service lookup is performed.

**Important:** `Entries` channel must be buffered or actively read. Sends are non-blocking — results are silently dropped if the channel is full.

#### Config

```go
type Config struct {
    Zone              Zone           // Zone to serve (required)
    Iface             *net.Interface // Bind to specific interface (optional)
    LogEmptyResponses bool           // Log when no records match a query
}
```

Server configuration.

### Client Functions

#### NewMDNSService

```go
func NewMDNSService(instance, service, domain, hostName string,
    portI interface{}, ipsI interface{}, txtI interface{}) (*MDNSService, error)
```

Creates a new mDNS service record. Flexible parameter types:

| Parameter  | Accepted types |
|------------|---------------|
| `portI`    | `int`, `*int` |
| `ipsI`     | `net.IP`, `*net.IP`, `[]net.IP`, `*[]net.IP`, `string`, `*string`, `[]string`, `*[]string` |
| `txtI`     | `string`, `[]string`, `*[]string` |

If `domain` is empty, defaults to `"local."`. If `hostName` is empty, uses `os.Hostname()`.

#### Query

```go
func Query(params *QueryParam) error
```

Performs a service lookup using the given parameters. Results are streamed to `params.Entries`. The function blocks until `params.Timeout` expires. Automatically creates and manages the mDNS client.

**Service matching modes:**

| Mode | Service format | Example | How it works |
|------|---------------|---------|-------------|
| Exact | `"_http._tcp"` | Queries for `_http._tcp.local.` PTR | Matches responses by suffix |
| Regex | `"~_.*._tcp"` | Queries for `_services._dns-sd._udp.local.` PTR (service enumeration) | Filters responses by Go regex |

In regex mode, the `~` prefix is stripped and the remaining string is used as a Go regexp pattern. The query first enumerates all services on the network, then filters results matching the pattern.

#### Lookup

```go
func Lookup(service string, entries chan<- *ServiceEntry) error
```

Convenience wrapper around `Query` with default parameters (domain: `"local"`, timeout: `1s`).

#### DefaultParams

```go
func DefaultParams(service string) *QueryParam
```

Returns a `QueryParam` with sensible defaults: domain `"local"`, timeout `1s`, unbuffered entries channel.

### Server Functions

#### NewServer

```go
func NewServer(config *Config) (*Server, error)
```

Creates and starts an mDNS server that listens for queries and responds with records from the configured `Zone`. Spawns background goroutines for IPv4 (and optionally IPv6) listeners.

#### Server.Shutdown

```go
func (s *Server) Shutdown() error
```

Gracefully stops the server and closes all listeners. Safe to call multiple times.

### Configuration Functions

#### InitPort

```go
func InitPort(port int)
```

Sets the mDNS port (default: `5353`). Must be called before `NewServer` or `Query`.

#### ConfigUseIpv6

```go
func ConfigUseIpv6(val bool)
```

Enables or disables IPv6 support (default: `false`). Must be called before `NewServer` or `Query`.

#### ConfigForceUnicast

```go
func ConfigForceUnicast(val bool)
```

Controls whether the server always sends unicast responses (default: `true`). When `false`, the server respects the unicast-response bit in queries per RFC 6762 §18.12.

#### InitMaddr

```go
func InitMaddr(ip4, ip6 string)
```

Sets custom multicast addresses (defaults: `224.0.0.251` for IPv4, `ff02::fb` for IPv6).

## Advanced Usage

### Custom Query with Timeout and Interface

```go
params := mdns.DefaultParams("_http._tcp")
params.Timeout = 5 * time.Second
params.Interface = iface // *net.Interface
params.Entries = make(chan *mdns.ServiceEntry, 16)

go func() {
    for entry := range params.Entries {
        fmt.Printf("%s at %s:%d\n", entry.Name, entry.AddrV4, entry.Port)
    }
}()

if err := mdns.Query(params); err != nil {
    log.Fatal(err)
}
close(params.Entries)
```

### Regex Service Discovery

Prefix the service name with `~` to switch from exact matching to regex filtering. This changes the query strategy:

1. **Without `~`** (exact): sends a PTR query for the specific service name → only receives responses for that service
2. **With `~`** (regex): sends a PTR query for `_services._dns-sd._udp.local.` → receives ALL advertised services on the network → filters responses using the regex pattern

```go
// Discover all TCP services on the network
params := mdns.DefaultParams("~_.*._tcp")
params.Timeout = 5 * time.Second
params.Entries = make(chan *mdns.ServiceEntry, 32)

go func() {
    for entry := range params.Entries {
        fmt.Printf("Found: %s at %s:%d\n", entry.Name, entry.AddrV4, entry.Port)
    }
}()

mdns.Query(params)
close(params.Entries)
```

More regex examples:

```go
// Match only HTTP and HTTPS services
mdns.DefaultParams("~_https?._tcp")

// Match any service containing "printer"
mdns.DefaultParams("~.*printer.*")

// Match services on a specific host pattern
mdns.DefaultParams("~myhost.*._http._tcp")
```

**Note:** Regex patterns use Go's `regexp` syntax. The pattern is matched against the full service instance name (e.g. `myhost._http._tcp.local.`).

**Matching examples** — given these services advertised on the network:

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
type myZone struct{}

func (z *myZone) Records(q dns.Question) []dns.RR {
    // Return custom DNS records based on the question
    return nil
}

server, _ := mdns.NewServer(&mdns.Config{Zone: &myZone{}})
defer server.Shutdown()
```

## Author

**sonnt85** — [thanhson.rf@gmail.com](mailto:thanhson.rf@gmail.com)

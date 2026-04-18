# mdns/relay

Pure-Go mDNS reflector. Forwards multicast DNS packets verbatim between
network interfaces — the standard pattern for making service discovery
work across broadcast domains (e.g. LAN ↔ Docker bridges).

**Sibling to:** [`avahi-daemon --enable-reflector`](https://linux.die.net/man/5/avahi-daemon.conf)
and [`mdns-repeater`](https://github.com/kennylevinsen/mdns-repeater). This
one is Go, dependency-free (beyond what the parent `mdns` package already
uses), and works from a single binary with zero config.

## Install

```bash
go install github.com/sonnt85/mdns/cmd/mdns-relay@latest

# typical usage
sudo mdns-relay -i eth0,docker0,br-*
```

## Library use

```go
import "github.com/sonnt85/mdns/relay"

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

r, err := relay.New(&relay.Config{
    Include: []string{"eth0", "docker0", "br-*"},
    Exclude: []string{"veth*", "docker_gwbridge"},
})
if err != nil { log.Fatal(err) }

if err := r.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
    log.Fatal(err)
}
```

## How it works

**Stateless reflector.** Each packet received on interface A is re-emitted
verbatim (same bytes, IP TTL=255) on every other active interface.

Three loop-prevention layers (all on by default):

1. **Self-IP filter** — drops packets from our own interface addresses.
2. **Per-interface echo guard** — never re-emits on the source interface.
3. **Dedup cache** — FNV-64a hash of packet bytes, 1s TTL.

## Why TTL=255 (verbatim)

RFC 6762 §11 requires TTL=255. Strict implementations (Apple Bonjour on
macOS/iOS) drop packets with TTL ≠ 255. Decrementing would break Apple
clients. We rely on the dedup cache instead.

## Docker use case

```
    ┌──────┐     eth0       ┌──────────────────┐
    │ iPad │ ───mDNS────►   │ Host (relay)     │
    └──────┘                │                  │
                            │  docker0         │
                            │   ├─ container1  │ ← now discovers iPad
                            │   └─ container2  │ ← and vice versa
                            │                  │
                            │  br-abc (compose)│
                            │   └─ service-app │ ← also reachable
                            └──────────────────┘
```

## Compatibility

- Linux: primary platform (bare-metal Docker, libvirt, LXC).
- macOS / Windows: builds, but Docker Desktop runs containers in a VM so
  the host-side relay doesn't help. Useful for other local bridges.
- IPv4 always. IPv6 optional via `EnableIPv6` / `-6`.

## Permissions

Multicast listening on port 5353 typically needs elevated privileges.

```bash
# option 1: run as root
sudo mdns-relay ...

# option 2: grant capability to the binary
sudo setcap cap_net_raw,cap_net_admin+ep $(which mdns-relay)
```

## Comparison

| | sonnt85/mdns/relay | avahi-reflector | mdns-repeater |
|-|:-:|:-:|:-:|
| Language | Go | C | C |
| Single binary | yes | no (daemon + libs) | yes |
| Dynamic interface watch | yes (glob) | no (manual restart) | no |
| Cross-platform build | yes | Linux-only | Linux-only |
| Dedup cache | yes (1s) | yes | no |

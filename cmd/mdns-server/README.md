# mdns-server

Publish an mDNS hostname and service, so other hosts on the LAN can reach
you by `<hostname>.local`. Designed to run inside a container alongside
[`mdns-relay`](../../relay/README.md) on the host.

## Install

```bash
go install github.com/sonnt85/mdns/cmd/mdns-server@latest
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | `os.Hostname()` | hostname to advertise (without `.local`) |
| `-ip` | `auto` | comma-separated IPs, or `auto` to detect non-loopback addrs |
| `-service` | `_workstation._tcp` | service type for PTR/SRV records |
| `-port` | `22` | service port in the SRV record |
| `-txt` | `mdns-server=<ver>` | TXT record (repeatable) |
| `-iface` | (all) | bind to a specific interface |
| `-domain` | `local` | mDNS domain |
| `-version` | | print version and exit |

## Container use case

```
┌────────────────────┐                ┌──────────────────┐
│ iPad 192.168.1.50  │                │ Host (relay)     │
│                    │                │  eth0 192.168.1.10
│  ping abc.local ──────mDNS query───→│                  │
│        ←─────── 172.17.0.7 ─────────│  docker0 172.17.0.1
│                    │                │   └─ container   │
│                    │                │      172.17.0.7  │
│                    │                │      ├─ mdns-server -host abc
│                    │                │      └─ advertises abc.local A 172.17.0.7
└────────────────────┘                └──────────────────┘
```

### 1. Run the relay on the host

```bash
sudo mdns-relay -i eth0,docker0,br-*
```

### 2. Run `mdns-server` in a container

```bash
docker build -t mdns-server -f cmd/mdns-server/Dockerfile .
docker run --rm -it \
    --name abc \
    -p 5353:5353/udp \
    mdns-server -host abc -ip auto
```

### 3. Ping from the LAN

```bash
# from any host on 192.168.1.0/24
ping abc.local
avahi-resolve-host-name abc.local
dig @224.0.0.251 -p 5353 abc.local
```

### 4. Resolve host's `.local` from inside the container

This does NOT require `mdns-server`. What the container needs is an mDNS
resolver. On Alpine/Debian:

```dockerfile
RUN apk add --no-cache nss-mdns  # or: libnss-mdns on Debian
```

And make sure `/etc/nsswitch.conf` includes `mdns4_minimal` before `dns`:

```
hosts: files mdns4_minimal [NOTFOUND=return] dns
```

Then, assuming the host also advertises its hostname via avahi/Bonjour AND
`mdns-relay` on the host reflects traffic into the container's bridge,
`ping <host>.local` from inside the container works.

## Permissions

Like any mDNS responder, binding to UDP `5353` on multicast may need:

```bash
sudo setcap cap_net_bind_service,cap_net_raw+ep $(which mdns-server)
```

In containers, running as root is usually simplest. The provided Dockerfile
does not drop privileges.

## Notes

- For `ping abc.local` to return the container IP, the container's
  reported IP (`-ip auto` picks its eth0 inside the bridge) must be
  routable from the querier. Docker's default bridge NATs outbound but
  container IPs (`172.17.0.x`) are NOT routable from the LAN — you need
  `mdns-relay` on the host to bridge the discovery AND a host-level port
  forward / `--network host` / reverse-proxy for actual connectivity.
- For LAN → container connectivity without port-forward, use
  `docker run --network host` and advertise the host's IP. Then
  `abc.local` resolves to the host, and any service on the host is
  reachable.

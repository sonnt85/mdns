// Package mdns is the root module for sonnt85/mdns. It contains no
// code; all functionality is exported from subpackages so importers
// pull only what they need:
//
//   - [github.com/sonnt85/mdns/service] — Service records + Zone
//     interface used to advertise services.
//   - [github.com/sonnt85/mdns/server] — mDNS multicast responder
//     that answers queries from a Zone.
//   - [github.com/sonnt85/mdns/client] — Query / Lookup for
//     discovering services on the local network.
//   - [github.com/sonnt85/mdns/relay] — stateless reflector that
//     forwards mDNS packets between interfaces (e.g. host ↔ docker
//     bridge).
//
// The repository also ships three CLIs under cmd/:
//
//   - mdns-grep   — regex-enabled discovery tool.
//   - mdns-server — one-shot service announcer.
//   - mdns-relay  — per-interface multicast relay.
//
// The root import path exposes no symbols; import the specific
// subpackage that matches your use case.
package mdns

// Package service implements mDNS service records and the Zone
// interface consumed by the server package. A Service instance
// describes a single announced service; server.Server calls
// Zone.Records to answer incoming mDNS questions from the network.
package service

import "github.com/miekg/dns"

// defaultTTL is the default TTL value in returned DNS records in seconds.
const defaultTTL = 120

// Zone is the interface used to integrate with the server and
// to serve records dynamically.
type Zone interface {
	// Records returns DNS records in response to a DNS question.
	Records(q dns.Question) []dns.RR
}

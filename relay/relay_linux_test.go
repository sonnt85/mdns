//go:build linux

package relay

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/sonnt85/mdns/client"
)

// needs: root, `ip` command
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration test needs root")
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("integration test needs `ip` command")
	}
}

func TestIntegrationRelayReflectsBetweenVeth(t *testing.T) {
	requireIntegration(t)

	// Setup: create two veth pairs, bring them up.
	// v1a ↔ v1b, v2a ↔ v2b. Relay listens on v1a and v2a.
	// Service published on v1b; discovery from v2b should find it
	// iff the relay is active.
	//
	// NOTE: this test is complex and requires careful cleanup. In a real
	// implementation, use `netns.New()` or a helper package. Here we
	// describe the shape; the engineer should implement using their
	// preferred netns library.
	t.Skip("TODO: implement veth/netns scaffold — requires a netns helper")
	_ = client.Lookup
	_ = dns.Fqdn
	_ = context.Background
	_ = time.Second
}

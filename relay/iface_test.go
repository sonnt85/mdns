package relay

import "testing"

func TestMatchInterface(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		exclude []string
		ifname  string
		want    bool
	}{
		{"exact match", []string{"eth0"}, nil, "eth0", true},
		{"exact miss", []string{"eth0"}, nil, "eth1", false},
		{"star suffix", []string{"br-*"}, nil, "br-abc123", true},
		{"star suffix miss", []string{"br-*"}, nil, "eth0", false},
		{"star prefix", []string{"*0"}, nil, "eth0", true},
		{"multi include", []string{"eth0", "docker0"}, nil, "docker0", true},
		{"exclude wins", []string{"eth*"}, []string{"eth1"}, "eth1", false},
		{"exclude pattern", []string{"*"}, []string{"veth*"}, "veth1234", false},
		{"exclude miss", []string{"eth*"}, []string{"veth*"}, "eth0", true},
		{"empty include", nil, nil, "eth0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchInterface(tt.ifname, tt.include, tt.exclude)
			if got != tt.want {
				t.Errorf("matchInterface(%q) = %v, want %v", tt.ifname, got, tt.want)
			}
		})
	}
}

func TestMatchInterfaceInvalidPattern(t *testing.T) {
	// filepath.Match returns error for "[" - we treat as no match
	got := matchInterface("eth0", []string{"["}, nil)
	if got {
		t.Errorf("invalid pattern should not match")
	}
}

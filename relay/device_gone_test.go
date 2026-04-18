package relay

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestIsDeviceGone(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ENODEV direct", syscall.ENODEV, true},
		{"ENETDOWN direct", syscall.ENETDOWN, true},
		{"ENETUNREACH direct", syscall.ENETUNREACH, true},
		{"wrapped ENODEV", fmt.Errorf("write udp4: sendto: %w", syscall.ENODEV), true},
		{"unrelated errno", syscall.EACCES, false},
		{"plain error", errors.New("random"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDeviceGone(tt.err)
			if got != tt.want {
				t.Errorf("isDeviceGone(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

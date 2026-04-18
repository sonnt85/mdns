// relay/config_test.go
package relay

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{Include: []string{"eth0"}}
	if err := cfg.applyDefaults(); err != nil {
		t.Fatalf("applyDefaults: %v", err)
	}
	if cfg.WatchInterval != 5*time.Second {
		t.Errorf("WatchInterval = %v, want 5s", cfg.WatchInterval)
	}
	if cfg.DedupWindow != time.Second {
		t.Errorf("DedupWindow = %v, want 1s", cfg.DedupWindow)
	}
	if cfg.Logger == nil {
		t.Error("Logger should default to log.Default().Printf")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{"empty include", &Config{}, true},
		{"valid single", &Config{Include: []string{"eth0"}}, false},
		{"valid with exclude", &Config{Include: []string{"eth*"}, Exclude: []string{"veth*"}}, false},
		{"negative watch", &Config{Include: []string{"eth0"}, WatchInterval: -1}, true},
		{"negative dedup", &Config{Include: []string{"eth0"}, DedupWindow: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.applyDefaults()
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

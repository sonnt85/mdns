// Command mdns-relay reflects mDNS packets between network interfaces.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sonnt85/mdns/relay"
)

var version = "dev"

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			*s = append(*s, p)
		}
	}
	return nil
}

func main() {
	var (
		include    stringsFlag
		exclude    stringsFlag
		v6         bool
		watch      time.Duration
		dedup      time.Duration
		verbose    bool
		statsEvery time.Duration
		showVer    bool
	)

	flag.Var(&include, "i", "interface patterns to include (comma-separated, glob)")
	flag.Var(&include, "iface", "alias of -i")
	flag.Var(&exclude, "x", "interface patterns to exclude (comma-separated, glob)")
	flag.Var(&exclude, "exclude", "alias of -x")
	flag.BoolVar(&v6, "6", false, "enable IPv6 relay")
	flag.DurationVar(&watch, "w", 5*time.Second, "interface watch interval (0 disables)")
	flag.DurationVar(&watch, "watch", 5*time.Second, "alias of -w")
	flag.DurationVar(&dedup, "d", time.Second, "dedup window")
	flag.DurationVar(&dedup, "dedup", time.Second, "alias of -d")
	flag.BoolVar(&verbose, "v", false, "log every forwarded packet")
	flag.BoolVar(&verbose, "verbose", false, "alias of -v")
	flag.DurationVar(&statsEvery, "s", 0, "print stats every N duration (0 disables)")
	flag.DurationVar(&statsEvery, "stats", 0, "alias of -s")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.Parse()

	if showVer {
		fmt.Println(version)
		return
	}
	if len(include) == 0 {
		fmt.Fprintln(os.Stderr, "error: -i is required (e.g. -i eth0,docker0,br-*)")
		flag.Usage()
		os.Exit(2)
	}
	if len(exclude) == 0 {
		exclude = []string{"veth*", "docker_gwbridge", "lo"}
	}

	logger := log.New(os.Stderr, "mdns-relay ", log.LstdFlags).Printf

	cfg := &relay.Config{
		Include:       include,
		Exclude:       exclude,
		EnableIPv6:    v6,
		WatchInterval: watch,
		DedupWindow:   dedup,
		Logger:        logger,
		Verbose:       verbose,
	}

	r, err := relay.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if statsEvery > 0 {
		go statsTicker(ctx, r, statsEvery, logger)
	}
	go usr1Stats(r, logger)

	if err := r.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "relay error: %v\n", err)
		hintIfPermErr(err)
		os.Exit(1)
	}
}

func statsTicker(ctx context.Context, r *relay.Relay, every time.Duration, logf func(string, ...any)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s := r.Stats()
			logf("stats: recv=%d fwd=%d drop_dedup=%d drop_self=%d ifaces=%d",
				s.PacketsReceived, s.PacketsForwarded,
				s.PacketsDroppedDedup, s.PacketsDroppedSelfEcho,
				s.ActiveInterfaces)
		}
	}
}

func hintIfPermErr(err error) {
	msg := err.Error()
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted") {
		fmt.Fprintln(os.Stderr, "hint: run as root or grant capability:")
		fmt.Fprintln(os.Stderr, "  sudo setcap cap_net_raw,cap_net_admin+ep $(which mdns-relay)")
	}
}

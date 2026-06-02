package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/guerro/pingtop/internal/pinger"
	"github.com/guerro/pingtop/internal/target"
	"github.com/guerro/pingtop/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pingtop:", err)
		os.Exit(1)
	}
}

func run() error {
	interval := flag.Duration("i", time.Second, "interval between pings")
	var maxHosts int
	flag.IntVar(&maxHosts, "max-hosts", 256, "hard cap on the number of expanded targets")
	flag.IntVar(&maxHosts, "m", 256, "alias for --max-hosts")
	var drop int
	flag.IntVar(&drop, "drop", 0, "drop a target after this many sends with no replies (0=disabled)")
	flag.IntVar(&drop, "d", 0, "alias for --drop")
	size := flag.Int("size", 24, "ICMP payload size in bytes")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: pingtop [flags] <target>...\n\n")
		fmt.Fprintf(os.Stderr, "Targets may be IPs, hostnames, or CIDR ranges.\n\nflags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return fmt.Errorf("no targets given")
	}

	mode, err := pinger.DetectMode()
	if err != nil {
		return err
	}

	targets, err := target.Expand(flag.Args(), maxHosts)
	if err != nil {
		return err
	}

	ids := make([]string, len(targets))
	for i, t := range targets {
		ids[i] = t.ID
	}

	// Buffer for ~4 events per target keeps emit() non-blocking under
	// typical load (1 Hz ping rate, sub-second UI redraw cadence).
	updates := make(chan pinger.StatsUpdate, len(targets)*4+1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for _, t := range targets {
		p := &pinger.Pinger{
			ID:       t.ID,
			Host:     t.Host,
			Mode:     mode,
			Interval: *interval,
			Size:     *size,
			Drop:     drop,
			Updates:  updates,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.Run(ctx)
		}()
	}

	// Safety net: if every pinger exits on its own (e.g., all targets
	// failed to resolve), close the channel so the UI sees EOF and
	// quits instead of hanging.
	go func() {
		wg.Wait()
		close(updates)
	}()

	prog := tea.NewProgram(ui.New(ids, updates), tea.WithAltScreen())
	_, runErr := prog.Run()

	cancel()
	wg.Wait()

	if runErr != nil {
		return fmt.Errorf("ui: %w", runErr)
	}
	return nil
}

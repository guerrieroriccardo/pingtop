package pinger

import (
	"context"
	"testing"
	"time"
)

// TestPingerLoopback is an integration smoke test: ping 127.0.0.1 and
// confirm we see at least one successful reply (Recv > 0 with non-zero
// RTT) within the deadline. Skipped if the host can't open ICMP
// sockets at all.
func TestPingerLoopback(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; skipped in -short")
	}
	mode, err := DetectMode()
	if err != nil {
		t.Skipf("no ICMP socket on this host: %v", err)
	}

	updates := make(chan StatsUpdate, 16)
	p := &Pinger{
		ID:       "127.0.0.1",
		Host:     "127.0.0.1",
		Mode:     mode,
		Interval: 100 * time.Millisecond,
		Size:     24,
		Updates:  updates,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case u := <-updates:
			if u.Recv > 0 && u.RTT > 0 && u.LastErr == nil {
				cancel()
				<-done
				return
			}
		case <-deadline:
			cancel()
			<-done
			t.Fatal("no successful reply from 127.0.0.1 within 2s")
		}
	}
}

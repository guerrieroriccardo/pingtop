// Package pinger probes ICMP reachability for a list of targets.
//
// This file owns socket-mode detection: whether the running process
// can use unprivileged SOCK_DGRAM ICMP (Linux's ping_group_range,
// macOS's default policy) or needs raw ICMP via CAP_NET_RAW / root.
// Detection happens once at startup so the per-target goroutines
// don't each pay the syscall cost.
package pinger

import (
	"errors"
	"fmt"
	"runtime"

	"golang.org/x/net/icmp"
)

// Mode is the ICMP socket flavour chosen for this process.
type Mode int

const (
	ModeUnprivileged Mode = iota // SOCK_DGRAM; no elevated privileges required
	ModePrivileged                // raw ICMP; needs CAP_NET_RAW or root
)

func (m Mode) String() string {
	switch m {
	case ModeUnprivileged:
		return "unprivileged"
	case ModePrivileged:
		return "privileged"
	default:
		return "unknown"
	}
}

// DetectMode probes the OS to pick the best available ICMP socket
// flavour. Preference order: unprivileged → privileged. Returns an
// error with actionable advice if neither works.
//
// Implementation note: rather than parsing /proc/sys/net/ipv4/ping_group_range
// or inspecting capabilities, we just try to open the socket. That
// picks up every relevant restriction (kernel sysctl, capabilities,
// LSM policy, container limits) uniformly and avoids OS-specific
// parsing.
func DetectMode() (Mode, error) {
	if probeUnprivileged() {
		return ModeUnprivileged, nil
	}
	if probePrivileged() {
		return ModePrivileged, nil
	}
	return 0, noSocketError()
}

func probeUnprivileged() bool {
	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func probePrivileged() bool {
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func noSocketError() error {
	switch runtime.GOOS {
	case "linux":
		return errors.New(
			"no ICMP socket available: re-run with sudo, or grant the " +
				"binary CAP_NET_RAW with `sudo setcap cap_net_raw+ep ./pingtop`, " +
				"or have an admin widen net.ipv4.ping_group_range",
		)
	case "darwin":
		return errors.New(
			"no ICMP socket available: re-run with sudo (unprivileged " +
				"ICMP usually works on macOS — check for unusual sandbox policy)",
		)
	default:
		return fmt.Errorf("no ICMP socket available on %s: re-run with elevated privileges", runtime.GOOS)
	}
}

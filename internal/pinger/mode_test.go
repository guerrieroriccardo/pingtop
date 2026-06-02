package pinger

import (
	"strings"
	"testing"
)

func TestModeString(t *testing.T) {
	for _, tc := range []struct {
		m    Mode
		want string
	}{
		{ModeUnprivileged, "unprivileged"},
		{ModePrivileged, "privileged"},
		{Mode(99), "unknown"},
	} {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

// TestDetectMode exercises the probe. The outcome depends on the host
// environment (Linux ping_group_range, capabilities, container limits),
// so the test only enforces the contract: either a valid mode with no
// error, or an actionable error containing the "no ICMP socket" hint.
func TestDetectMode(t *testing.T) {
	mode, err := DetectMode()
	if err != nil {
		if !strings.Contains(err.Error(), "no ICMP socket available") {
			t.Errorf("error should mention 'no ICMP socket available', got: %v", err)
		}
		t.Logf("DetectMode returned err (likely sandboxed env): %v", err)
		return
	}
	if mode != ModeUnprivileged && mode != ModePrivileged {
		t.Errorf("DetectMode returned invalid mode %v", mode)
	}
	t.Logf("DetectMode picked %s", mode)
}

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/guerrieroriccardo/pingtop/internal/pinger"
)

func TestFormatRTT(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    pinger.StatsUpdate
		want string
	}{
		{"no data", pinger.StatsUpdate{}, "—"},
		{"with rtt", pinger.StatsUpdate{RTT: 2*time.Millisecond + 500*time.Microsecond}, "2.5ms"},
		{"err sticky", pinger.StatsUpdate{LastErr: errors.New("boom")}, "err"},
		{"rtt overrides err", pinger.StatsUpdate{RTT: time.Millisecond, LastErr: errors.New("boom")}, "1ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatRTT(tc.s); got != tc.want {
				t.Errorf("formatRTT = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatJitter(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    pinger.StatsUpdate
		want string
	}{
		{"no data", pinger.StatsUpdate{}, "—"},
		{"with jitter", pinger.StatsUpdate{Jitter: 750 * time.Microsecond}, "750µs"},
		{"err has no effect", pinger.StatsUpdate{LastErr: errors.New("boom")}, "—"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatJitter(tc.s); got != tc.want {
				t.Errorf("formatJitter = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatLoss(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    pinger.StatsUpdate
		want string
	}{
		{"no sends yet", pinger.StatsUpdate{}, "—"},
		{"all received", pinger.StatsUpdate{Sent: 10, Recv: 10}, "0.0%"},
		{"half lost", pinger.StatsUpdate{Sent: 10, Recv: 5}, "50.0%"},
		{"all lost", pinger.StatsUpdate{Sent: 5, Recv: 0}, "100.0%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatLoss(tc.s); got != tc.want {
				t.Errorf("formatLoss = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatSentLost(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    pinger.StatsUpdate
		want string
	}{
		{"no data", pinger.StatsUpdate{}, "—"},
		{"all received", pinger.StatsUpdate{Sent: 10, Recv: 10}, "10/0"},
		{"two lost", pinger.StatsUpdate{Sent: 10, Recv: 8}, "10/2"},
		{"recv exceeds sent clamps", pinger.StatsUpdate{Sent: 5, Recv: 6}, "5/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatSentLost(tc.s); got != tc.want {
				t.Errorf("formatSentLost = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildRowsInitial(t *testing.T) {
	rows := buildRows([]string{"1.1.1.1", "8.8.8.8"}, nil, nil)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "1.1.1.1" || rows[1][0] != "8.8.8.8" {
		t.Errorf("rows lost their order: %v", rows)
	}
	for _, r := range rows {
		if len(r) != 6 || r[1] != "—" || r[2] != "—" || r[3] != "—" || r[4] != "—" {
			t.Errorf("expected placeholders, got %v", r)
		}
	}
}

func TestUpdateApplyingStatsMsg(t *testing.T) {
	updates := make(chan pinger.StatsUpdate, 4)
	m := New([]string{"1.1.1.1", "8.8.8.8"}, updates)

	mm, _ := m.Update(statsMsg{TargetID: "1.1.1.1", Sent: 4, Recv: 3, RTT: 5 * time.Millisecond})
	got := mm.(Model).stats["1.1.1.1"]
	if got.RTT != 5*time.Millisecond || got.Sent != 4 || got.Recv != 3 {
		t.Errorf("stats not stored as expected: %+v", got)
	}

	view := mm.(Model).View()
	if !strings.Contains(view, "5ms") || !strings.Contains(view, "25.0%") {
		t.Errorf("view missing expected formatted values:\n%s", view)
	}
}

func TestUpdateRemovesDroppedTarget(t *testing.T) {
	updates := make(chan pinger.StatsUpdate, 4)
	m := New([]string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}, updates)

	mm, _ := m.Update(statsMsg{TargetID: "8.8.8.8", Dropped: true})
	out := mm.(Model)

	if len(out.order) != 2 || out.order[0] != "1.1.1.1" || out.order[1] != "9.9.9.9" {
		t.Errorf("order should be [1.1.1.1 9.9.9.9], got %v", out.order)
	}
	if _, ok := out.stats["8.8.8.8"]; ok {
		t.Errorf("stats for dropped target should be removed")
	}
	view := out.View()
	if strings.Contains(view, "8.8.8.8") {
		t.Errorf("view should not show dropped target:\n%s", view)
	}
	if !strings.Contains(view, "1.1.1.1") || !strings.Contains(view, "9.9.9.9") {
		t.Errorf("view should still show survivors:\n%s", view)
	}
}

func TestUpdateQuitOnKey(t *testing.T) {
	updates := make(chan pinger.StatsUpdate)
	m := New([]string{"1.1.1.1"}, updates)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should produce a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q should produce tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateQuitOnClosedChannel(t *testing.T) {
	updates := make(chan pinger.StatsUpdate)
	close(updates)
	m := New([]string{"1.1.1.1"}, updates)

	cmd := m.Init()
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("closed channel should produce tea.QuitMsg, got %T", cmd())
	}
}

func TestFilterMatchesSubstring(t *testing.T) {
	updates := make(chan pinger.StatsUpdate)
	m := New([]string{"1.1.1.1", "8.8.8.8", "192.168.1.10"}, updates)

	m.filter = "8.8"
	v := m.visibleIDs()
	if len(v) != 1 || v[0] != "8.8.8.8" {
		t.Errorf("expected [8.8.8.8] for filter %q, got %v", m.filter, v)
	}

	m.filter = "."
	v = m.visibleIDs()
	if len(v) != 3 {
		t.Errorf("expected all 3 to match filter %q, got %v", m.filter, v)
	}

	m.filter = "xyz"
	v = m.visibleIDs()
	if len(v) != 0 {
		t.Errorf("expected no matches for filter %q, got %v", m.filter, v)
	}
}

func TestFilterCaseInsensitive(t *testing.T) {
	updates := make(chan pinger.StatsUpdate)
	m := New([]string{"host-A.example", "HOST-b.example"}, updates)

	m.filter = "host-a"
	v := m.visibleIDs()
	if len(v) != 1 || v[0] != "host-A.example" {
		t.Errorf("expected [host-A.example] for filter %q, got %v", m.filter, v)
	}
}

func TestFormatSparkEmpty(t *testing.T) {
	got := formatSpark(nil)
	if got != strings.Repeat(" ", sparkWidth) {
		t.Errorf("empty history should render as %d spaces, got %q", sparkWidth, got)
	}
}

func TestFormatSparkAllEqual(t *testing.T) {
	h := []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	got := formatSpark(h)
	mid := string(sparkBars[len(sparkBars)/2])
	// Three middle bars, padded on the left to sparkWidth.
	want := strings.Repeat(" ", sparkWidth-3) + strings.Repeat(mid, 3)
	if got != want {
		t.Errorf("equal samples should all map to middle bar\n got=%q\nwant=%q", got, want)
	}
}

func TestFormatSparkScalesMinMax(t *testing.T) {
	h := []time.Duration{1 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond}
	got := formatSpark(h)
	runes := []rune(got)
	// Last three runes are the data; min should be first bar, max should be last bar.
	last3 := runes[len(runes)-3:]
	if last3[0] != sparkBars[0] {
		t.Errorf("min sample should map to %c, got %c", sparkBars[0], last3[0])
	}
	if last3[2] != sparkBars[len(sparkBars)-1] {
		t.Errorf("max sample should map to %c, got %c", sparkBars[len(sparkBars)-1], last3[2])
	}
}

func TestAppendHistoryRingBuffer(t *testing.T) {
	h := make(map[string][]time.Duration)
	for i := 0; i < sparkWidth+5; i++ {
		appendHistory(h, "x", time.Duration(i)*time.Millisecond)
	}
	if len(h["x"]) != sparkWidth {
		t.Errorf("history should cap at %d samples, got %d", sparkWidth, len(h["x"]))
	}
	// The oldest 5 samples should have been evicted; the buffer's first
	// sample should be sample #5 (zero-indexed).
	if h["x"][0] != 5*time.Millisecond {
		t.Errorf("oldest sample should be 5ms, got %v", h["x"][0])
	}
}

func TestUpdateAppendsHistoryOnRTT(t *testing.T) {
	updates := make(chan pinger.StatsUpdate, 4)
	m := New([]string{"1.1.1.1"}, updates)

	mm, _ := m.Update(statsMsg{TargetID: "1.1.1.1", Sent: 1, Recv: 1, RTT: 3 * time.Millisecond})
	out := mm.(Model)
	if len(out.history["1.1.1.1"]) != 1 || out.history["1.1.1.1"][0] != 3*time.Millisecond {
		t.Errorf("expected one 3ms sample, got %v", out.history["1.1.1.1"])
	}

	// An RTT=0 message (OnSend snapshot) should NOT append.
	mm, _ = out.Update(statsMsg{TargetID: "1.1.1.1", Sent: 2, Recv: 1, RTT: 0})
	out = mm.(Model)
	if len(out.history["1.1.1.1"]) != 1 {
		t.Errorf("RTT=0 message should not append, got %v", out.history["1.1.1.1"])
	}
}

func TestViewWhenAllTargetsDropped(t *testing.T) {
	updates := make(chan pinger.StatsUpdate, 4)
	m := New([]string{"1.1.1.1"}, updates)

	mm, _ := m.Update(statsMsg{TargetID: "1.1.1.1", Dropped: true})
	view := mm.(Model).View()
	if !strings.Contains(view, "no hosts reachable") {
		t.Errorf("view should show empty-state message, got:\n%s", view)
	}
}

func TestFilterUpdateOnSlashKey(t *testing.T) {
	updates := make(chan pinger.StatsUpdate, 4)
	m := New([]string{"1.1.1.1", "8.8.8.8"}, updates)

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	out := mm.(Model)
	if !out.filterMode {
		t.Fatalf("expected filterMode=true after '/'")
	}

	mm, _ = out.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'8'}})
	out = mm.(Model)
	mm, _ = out.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	out = mm.(Model)
	if out.filter != "8." {
		t.Errorf("expected filter=%q, got %q", "8.", out.filter)
	}

	mm, _ = out.Update(tea.KeyMsg{Type: tea.KeyEsc})
	out = mm.(Model)
	if out.filterMode {
		t.Errorf("expected filterMode=false after esc")
	}
	if out.filter != "" {
		t.Errorf("expected filter cleared after esc, got %q", out.filter)
	}
}

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/guerro/pingtop/internal/pinger"
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
	rows := buildRows([]string{"1.1.1.1", "8.8.8.8"}, nil)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "1.1.1.1" || rows[1][0] != "8.8.8.8" {
		t.Errorf("rows lost their order: %v", rows)
	}
	for _, r := range rows {
		if len(r) != 5 || r[1] != "—" || r[2] != "—" || r[3] != "—" || r[4] != "—" {
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

// Package ui renders the pingtop dashboard with Bubble Tea.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/guerrieroriccardo/pingtop/internal/pinger"
)

// Column widths are fixed; lipgloss/table reads them from the StyleFunc
// via lipgloss.Style.GetWidth(). Order matches the cells emitted by
// buildRows.
var columnWidths = []int{28, 10, 10, 10, 10, 10, 8, 12, sparkWidth + 2}
var columnHeaders = []string{"TARGET", "RTT", "MIN", "AVG", "MAX", "JITTER", "LOSS%", "SENT/LOST", "SPARK"}

// statsMsg wraps a StatsUpdate so it can flow through the Bubble Tea
// message bus without exposing pinger types as a top-level tea.Msg.
type statsMsg pinger.StatsUpdate

// Model is the dashboard state. Construct it with New, then pass it to
// tea.NewProgram.
type Model struct {
	order       []string
	updates     <-chan pinger.StatsUpdate
	stats       map[string]pinger.StatsUpdate
	history     map[string][]time.Duration // per-target RTT ring buffer for the sparkline
	termHeight  int                        // last WindowSizeMsg height; 0 until first event
	offset      int                        // first row index shown when content overflows viewport
	filterMode  bool                       // true while user is typing into the filter
	filter      string                     // active filter; empty == no filter
	keepDropped bool                       // if true, dropped rows stay visible with final stats
	styler      styler                     // colors for RTT/JITTER/LOSS%; disabled styler is a no-op
}

// New builds the initial model. ids is the stable display order
// produced by target.Expand; updates is the shared channel the
// pingers publish on. If keepDropped is true, rows for targets that
// hit the drop threshold stay visible with their final (100% loss)
// stats instead of being removed. If colorize is true, the RTT,
// JITTER, and LOSS% cells are tinted by threshold.
func New(ids []string, updates <-chan pinger.StatsUpdate, keepDropped, colorize bool) Model {
	return Model{
		order:       ids,
		updates:     updates,
		stats:       make(map[string]pinger.StatsUpdate, len(ids)),
		history:     make(map[string][]time.Duration, len(ids)),
		keepDropped: keepDropped,
		styler:      newStyler(colorize),
	}
}

func (m Model) Init() tea.Cmd {
	return m.waitForUpdate()
}

// waitForUpdate returns a Cmd that blocks on the updates channel and
// turns the next event into a statsMsg. When the channel is closed
// (main has stopped all pingers) it emits tea.Quit so the program
// exits cleanly.
func (m Model) waitForUpdate() tea.Cmd {
	ch := m.updates
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return statsMsg(u)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case statsMsg:
		if msg.Dropped {
			if m.keepDropped {
				// Persist the final snapshot (Sent=N, Recv=0) so the
				// row keeps showing 100% loss after the pinger stops.
				m.stats[msg.TargetID] = pinger.StatsUpdate(msg)
			} else {
				delete(m.stats, msg.TargetID)
				delete(m.history, msg.TargetID)
				m.order = removeID(m.order, msg.TargetID)
				clampOffset(&m)
			}
			return m, m.waitForUpdate()
		}
		m.stats[msg.TargetID] = pinger.StatsUpdate(msg)
		if msg.RTT > 0 {
			appendHistory(m.history, msg.TargetID, msg.RTT)
		}
		return m, m.waitForUpdate()

	case tea.KeyMsg:
		if m.filterMode {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.filterMode = false
				m.filter = ""
				clampOffset(&m)
			case tea.KeyEnter:
				m.filterMode = false
			case tea.KeyBackspace:
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					clampOffset(&m)
				}
			case tea.KeySpace:
				m.filter += " "
				clampOffset(&m)
			case tea.KeyRunes:
				m.filter += string(msg.Runes)
				clampOffset(&m)
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.filterMode = true
			return m, nil
		case "esc":
			if m.filter != "" {
				m.filter = ""
				clampOffset(&m)
			}
			return m, nil
		case "up", "k":
			if m.offset > 0 {
				m.offset--
			}
			return m, nil
		case "down", "j":
			if m.offset < m.maxOffset() {
				m.offset++
			}
			return m, nil
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		clampOffset(&m)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var text string
	switch {
	case m.filterMode:
		text = fmt.Sprintf("/%s█  [enter] apply  [esc] clear", m.filter)
	case len(m.order) == 0:
		text = "no hosts reachable — [q] quit"
	case m.filter != "":
		text = fmt.Sprintf("filter: %s  [/] edit  [esc] clear  [q] quit", m.filter)
	default:
		text = "[q] quit  [/] filter  [↑/↓] scroll"
	}
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(text)
	return m.renderTable() + "\n" + help
}

// renderTable builds a fresh lipgloss/table on every call. lipgloss/table
// is a stateless renderer, not a Bubble Tea component, so we rebuild from
// current model state rather than holding a long-lived table instance.
func (m Model) renderTable() string {
	rows := buildRows(m.visibleIDs(), m.stats, m.history, m.styler)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(true).
		BorderStyle(dim).
		Headers(columnHeaders...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().
				Width(columnWidths[col]).
				MaxWidth(columnWidths[col]).
				PaddingRight(1)
			if row == table.HeaderRow {
				s = s.Bold(true)
			}
			return s
		})
	// Height() enables lipgloss/table's offset-based scroll. Without it
	// the table tries to render every row, which overflows the terminal
	// on large CIDR scans.
	if m.termHeight > 0 {
		// Reserve one line for the help text below the table.
		t = t.Height(m.termHeight - 1)
	}
	return t.Offset(m.offset).String()
}

// visibleIDs returns m.order filtered by m.filter (case-insensitive
// substring). When the filter is empty, it returns m.order directly.
func (m Model) visibleIDs() []string {
	if m.filter == "" {
		return m.order
	}
	f := strings.ToLower(m.filter)
	out := make([]string, 0, len(m.order))
	for _, id := range m.order {
		if strings.Contains(strings.ToLower(id), f) {
			out = append(out, id)
		}
	}
	return out
}

// buildRows produces table rows in the stable order. A nil stats map
// renders all targets in their initial "no data yet" state.
func buildRows(order []string, stats map[string]pinger.StatsUpdate, history map[string][]time.Duration, st styler) [][]string {
	rows := make([][]string, len(order))
	for i, id := range order {
		s, ok := stats[id]
		if !ok {
			rows[i] = []string{id, "—", "—", "—", "—", "—", "—", "—", formatSpark(nil)}
			continue
		}
		rows[i] = []string{
			id,
			st.render(formatRTT(s), rttLevel(s)),
			formatDur(s.MinRTT),
			formatDur(s.AvgRTT),
			formatDur(s.MaxRTT),
			st.render(formatJitter(s), jitterLevel(s)),
			st.render(formatLoss(s), lossLevel(s)),
			formatSentLost(s),
			formatSpark(history[id]),
		}
	}
	return rows
}

func formatRTT(s pinger.StatsUpdate) string {
	if s.RTT > 0 {
		return s.RTT.Round(time.Microsecond).String()
	}
	if s.LastErr != nil {
		return "err"
	}
	return "—"
}

func formatJitter(s pinger.StatsUpdate) string {
	if s.Jitter > 0 {
		return s.Jitter.Round(time.Microsecond).String()
	}
	return "—"
}

// formatDur renders a Duration as a microsecond-rounded string, or "—"
// for zero. Used by the MIN/AVG/MAX columns which have no error branch.
func formatDur(d time.Duration) string {
	if d > 0 {
		return d.Round(time.Microsecond).String()
	}
	return "—"
}

func formatLoss(s pinger.StatsUpdate) string {
	if s.Sent == 0 {
		return "—"
	}
	pct := 100 * float64(s.Sent-s.Recv) / float64(s.Sent)
	return fmt.Sprintf("%.1f%%", pct)
}

func formatSentLost(s pinger.StatsUpdate) string {
	if s.Sent == 0 {
		return "—"
	}
	lost := s.Sent - s.Recv
	if lost < 0 {
		lost = 0
	}
	return fmt.Sprintf("%d/%d", s.Sent, lost)
}

// level classifies a metric value into a color bucket. levelNeutral
// means "no data" / "no verdict" — styler renders it without color.
type level int

const (
	levelNeutral level = iota
	levelGood
	levelWarn
	levelCrit
)

// Threshold defaults: chosen to match common sysadmin intuition for
// LAN/WAN ping monitoring. Not configurable in v0.10; promote to flags
// if anyone asks.
const (
	rttWarn     = 50 * time.Millisecond
	rttCrit     = 200 * time.Millisecond
	jitterWarn  = 5 * time.Millisecond
	jitterCrit  = 20 * time.Millisecond
	lossCritPct = 5.0 // ≥ 5% loss is crit; anything > 0 and < 5 is warn
)

func rttLevel(s pinger.StatsUpdate) level {
	if s.RTT > 0 {
		switch {
		case s.RTT < rttWarn:
			return levelGood
		case s.RTT < rttCrit:
			return levelWarn
		default:
			return levelCrit
		}
	}
	if s.LastErr != nil {
		return levelCrit
	}
	return levelNeutral
}

func jitterLevel(s pinger.StatsUpdate) level {
	if s.Jitter <= 0 {
		return levelNeutral
	}
	switch {
	case s.Jitter < jitterWarn:
		return levelGood
	case s.Jitter < jitterCrit:
		return levelWarn
	default:
		return levelCrit
	}
}

func lossLevel(s pinger.StatsUpdate) level {
	if s.Sent == 0 {
		return levelNeutral
	}
	pct := 100 * float64(s.Sent-s.Recv) / float64(s.Sent)
	switch {
	case pct == 0:
		return levelGood
	case pct < lossCritPct:
		return levelWarn
	default:
		return levelCrit
	}
}

// styler wraps cell strings in lipgloss color styles. The zero value
// (enabled=false) is a no-op renderer, so passing styler{} disables
// coloring without special-casing callers.
type styler struct {
	enabled    bool
	good, warn, crit lipgloss.Style
}

func newStyler(enabled bool) styler {
	if !enabled {
		return styler{}
	}
	return styler{
		enabled: true,
		good:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		crit:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
}

func (st styler) render(s string, l level) string {
	if !st.enabled || l == levelNeutral {
		return s
	}
	switch l {
	case levelGood:
		return st.good.Render(s)
	case levelWarn:
		return st.warn.Render(s)
	case levelCrit:
		return st.crit.Render(s)
	}
	return s
}

// headerLines counts the rows lipgloss/table renders above the data:
// one for the header titles and one for the border-bottom separator.
const headerLines = 2

// maxOffset is the highest offset that still keeps the last row in
// view. Below this, every row is visible without scrolling.
func (m Model) maxOffset() int {
	if m.termHeight <= 0 {
		return 0
	}
	// Available data rows = termHeight - help line - header lines.
	avail := m.termHeight - 1 - headerLines
	if avail < 1 {
		avail = 1
	}
	rows := len(m.visibleIDs())
	if rows <= avail {
		return 0
	}
	return rows - avail
}

// clampOffset reins offset back in after the visible set or terminal
// height changes (filter edit, dropped row, window resize). Called from
// Update; safe to invoke whenever m.offset might have gone stale.
func clampOffset(m *Model) {
	if max := m.maxOffset(); m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func removeID(order []string, id string) []string {
	for i, s := range order {
		if s == id {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

// sparkWidth is the number of recent RTT samples shown in the SPARK
// column. At a 1 s interval this is also the seconds of visible history.
const sparkWidth = 20

// sparkBars is the 8-level Unicode bar set used to render samples.
var sparkBars = []rune("▁▂▃▄▅▆▇█")

func appendHistory(h map[string][]time.Duration, id string, rtt time.Duration) {
	buf := h[id]
	if len(buf) >= sparkWidth {
		buf = buf[1:]
	}
	h[id] = append(buf, rtt)
}

// formatSpark renders the recent RTT samples as a Unicode bar chart,
// scaled per-target between the window's min and max so relative
// jitter is what's visible. Pads with leading spaces until the buffer
// fills, so the latest sample is always at the right edge.
func formatSpark(history []time.Duration) string {
	if len(history) == 0 {
		return strings.Repeat(" ", sparkWidth)
	}
	min, max := history[0], history[0]
	for _, d := range history[1:] {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	rng := max - min

	var b strings.Builder
	b.Grow(sparkWidth * 4) // bars are 3-byte UTF-8 runes
	for i := 0; i < sparkWidth-len(history); i++ {
		b.WriteByte(' ')
	}
	for _, d := range history {
		idx := len(sparkBars) / 2
		if rng > 0 {
			idx = int(int64(d-min) * int64(len(sparkBars)-1) / int64(rng))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(sparkBars) {
				idx = len(sparkBars) - 1
			}
		}
		b.WriteRune(sparkBars[idx])
	}
	return b.String()
}

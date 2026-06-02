// Package ui renders the pingtop dashboard with Bubble Tea.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/guerro/pingtop/internal/pinger"
)

// statsMsg wraps a StatsUpdate so it can flow through the Bubble Tea
// message bus without exposing pinger types as a top-level tea.Msg.
type statsMsg pinger.StatsUpdate

// Model is the dashboard state. Construct it with New, then pass it to
// tea.NewProgram.
type Model struct {
	order      []string
	updates    <-chan pinger.StatsUpdate
	table      table.Model
	stats      map[string]pinger.StatsUpdate
	termHeight int    // last WindowSizeMsg height, used to re-clamp on drop
	filterMode bool   // true while user is typing into the filter
	filter     string // active filter; empty == no filter
}

// New builds the initial model. ids is the stable display order
// produced by target.Expand; updates is the shared channel the
// pingers publish on.
func New(ids []string, updates <-chan pinger.StatsUpdate) Model {
	columns := []table.Column{
		{Title: "TARGET", Width: 28},
		{Title: "RTT", Width: 12},
		{Title: "JITTER", Width: 10},
		{Title: "LOSS%", Width: 8},
		{Title: "SENT/LOST", Width: 12},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(buildRows(ids, nil)),
		table.WithFocused(true),
		table.WithHeight(initialHeight(len(ids))),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return Model{
		order:   ids,
		updates: updates,
		table:   t,
		stats:   make(map[string]pinger.StatsUpdate, len(ids)),
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
			delete(m.stats, msg.TargetID)
			m.order = removeID(m.order, msg.TargetID)
			refresh(&m)
			return m, m.waitForUpdate()
		}
		m.stats[msg.TargetID] = pinger.StatsUpdate(msg)
		refresh(&m)
		return m, m.waitForUpdate()

	case tea.KeyMsg:
		if m.filterMode {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.filterMode = false
				m.filter = ""
				refresh(&m)
			case tea.KeyEnter:
				m.filterMode = false
			case tea.KeyBackspace:
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					refresh(&m)
				}
			case tea.KeySpace:
				m.filter += " "
				refresh(&m)
			case tea.KeyRunes:
				m.filter += string(msg.Runes)
				refresh(&m)
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
				refresh(&m)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		refresh(&m)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var text string
	switch {
	case m.filterMode:
		text = fmt.Sprintf("/%s█  [enter] apply  [esc] clear", m.filter)
	case m.filter != "":
		text = fmt.Sprintf("filter: %s  [/] edit  [esc] clear  [q] quit", m.filter)
	default:
		text = "[q] quit  [/] filter  [↑/↓] scroll"
	}
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(text)
	return m.table.View() + "\n" + help
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

// refresh rebuilds the table rows + height from the current filter and
// stats. Pointer receiver because the table is a value field and we
// need its internal state updates to stick on the caller's Model.
func refresh(m *Model) {
	v := m.visibleIDs()
	m.table.SetRows(buildRows(v, m.stats))
	if m.termHeight > 0 {
		m.table.SetHeight(clampHeight(m.termHeight, len(v)))
	}
}

// buildRows produces table rows in the stable order. A nil stats map
// renders all targets in their initial "no data yet" state.
func buildRows(order []string, stats map[string]pinger.StatsUpdate) []table.Row {
	rows := make([]table.Row, len(order))
	for i, id := range order {
		s, ok := stats[id]
		if !ok {
			rows[i] = table.Row{id, "—", "—", "—", "—"}
			continue
		}
		rows[i] = table.Row{id, formatRTT(s), formatJitter(s), formatLoss(s), formatSentLost(s)}
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

// headerLines is the rendered height of bubbles/table's header
// (title row + border-bottom). SetHeight values must add this on top
// of the desired data-row count.
const headerLines = 2

func initialHeight(n int) int {
	if n < 1 {
		return headerLines + 1
	}
	return n + headerLines
}

// clampHeight returns a SetHeight value bounded by the terminal height
// (minus one line for the help text) and the actual row count, so the
// table neither overflows the screen nor reserves empty rows below
// surviving targets after a drop.
func clampHeight(termHeight, rows int) int {
	h := termHeight - 1
	if h < headerLines+1 {
		h = headerLines + 1
	}
	if max := rows + headerLines; h > max {
		h = max
	}
	return h
}

func removeID(order []string, id string) []string {
	for i, s := range order {
		if s == id {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

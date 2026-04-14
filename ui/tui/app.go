// Package tui provides the terminal UI for pbmon using bubbletea + lipgloss.
package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/internal/model"
	"github.com/Ivlyth/process-bandwidth/internal/store"
)

// ──────────────────────────────────────────────────────────────
// Styles
// ──────────────────────────────────────────────────────────────

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	rxStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))  // green
	txStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // orange
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// ──────────────────────────────────────────────────────────────
// Sort options
// ──────────────────────────────────────────────────────────────

type sortBy int

const (
	sortByNetRx sortBy = iota
	sortByNetTx
	sortByFileRead
	sortByFileWrite
	sortByPID
	sortByName
	sortByCount
)

func (s sortBy) String() string {
	switch s {
	case sortByNetRx:
		return "Net↓"
	case sortByNetTx:
		return "Net↑"
	case sortByFileRead:
		return "File Read"
	case sortByFileWrite:
		return "File Write"
	case sortByPID:
		return "PID"
	case sortByName:
		return "Name"
	}
	return "?"
}

// ──────────────────────────────────────────────────────────────
// Active panel
// ──────────────────────────────────────────────────────────────

type panel int

const (
	panelProcess panel = iota
	panelConn
)

// ──────────────────────────────────────────────────────────────
// Messages
// ──────────────────────────────────────────────────────────────

type tickMsg time.Time
type quitMsg struct{}

// ──────────────────────────────────────────────────────────────
// Model
// ──────────────────────────────────────────────────────────────

// AppModel is the top-level bubbletea model for the TUI.
type AppModel struct {
	cfg   *config.Config
	store *store.Store
	keys  keyMap

	// State
	activePanel panel
	procCursor  int
	connCursor  int
	sortMode    sortBy
	filterStr   string
	filtering   bool
	paused      bool
	showFileIO  bool
	showHelp    bool
	width       int
	height      int

	// Cached snapshot (updated on tick)
	procs []*model.Process
	conns []*model.Connection // connections of selected process
}

// Start creates and runs the bubbletea TUI. It blocks until the TUI exits,
// then calls cancel() to trigger graceful shutdown of the rest of the app.
// ctx is used so that an external signal (SIGTERM) also causes the TUI to exit.
func Start(ctx context.Context, cancel context.CancelFunc, cfg *config.Config, s *store.Store) error {
	m := &AppModel{
		cfg:        cfg,
		store:      s,
		keys:       defaultKeys,
		sortMode:   sortByNetRx,
		showFileIO: cfg.IncludeFileIO,
		width:      120,
		height:     40,
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)
	_, err := p.Run()
	// Always signal shutdown after the TUI exits (q, ctrl+c, or external signal).
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil // expected when ctx is cancelled externally
	}
	return err
}

// ──────────────────────────────────────────────────────────────
// bubbletea interface
// ──────────────────────────────────────────────────────────────

func (m AppModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if !m.paused {
			m.refresh()
		}
		return m, tickCmd()

	case quitMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filtering mode: collect characters
	if m.filtering {
		switch msg.String() {
		case "enter", "esc":
			m.filtering = false
		case "backspace":
			if len(m.filterStr) > 0 {
				m.filterStr = m.filterStr[:len(m.filterStr)-1]
			}
		default:
			if len(msg.Runes) > 0 {
				m.filterStr += string(msg.Runes)
			}
		}
		m.refresh()
		return m, nil
	}

	switch {
	case msg.String() == "q" || msg.String() == "ctrl+c":
		return m, tea.Quit

	case msg.String() == "tab":
		if m.activePanel == panelProcess {
			m.activePanel = panelConn
		} else {
			m.activePanel = panelProcess
		}

	case msg.String() == "up" || msg.String() == "k":
		if m.activePanel == panelProcess {
			if m.procCursor > 0 {
				m.procCursor--
			}
		} else {
			if m.connCursor > 0 {
				m.connCursor--
			}
		}

	case msg.String() == "down" || msg.String() == "j":
		if m.activePanel == panelProcess {
			if m.procCursor < len(m.procs)-1 {
				m.procCursor++
			}
		} else {
			if m.connCursor < len(m.conns)-1 {
				m.connCursor++
			}
		}

	case msg.String() == "s":
		m.sortMode = (m.sortMode + 1) % sortByCount
		m.sortProcs()

	case msg.String() == "/":
		m.filtering = true

	case msg.String() == "f":
		m.showFileIO = !m.showFileIO

	case msg.String() == "p" || msg.String() == "F5":
		m.paused = !m.paused

	case msg.String() == "?":
		m.showHelp = !m.showHelp
	}

	m.updateConnList()
	return m, nil
}

func (m AppModel) View() string {
	var sb strings.Builder

	// Title bar
	title := titleStyle.Render("pbmon") + dimStyle.Render(fmt.Sprintf(
		"  processes:%d  sort:%s  ", m.store.ProcessCount(), m.sortMode))
	if m.paused {
		title += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[PAUSED]")
	}
	if m.showFileIO {
		title += dimStyle.Render("  [file-io on]")
	}
	sb.WriteString(title + "\n")

	// Filter bar
	if m.filtering || m.filterStr != "" {
		sb.WriteString(dimStyle.Render("filter: ") + m.filterStr)
		if m.filtering {
			sb.WriteString("█")
		}
		sb.WriteString("\n")
	}

	// Main content: process table (left) | connection table (right)
	procHeight := m.height - 5
	if procHeight < 5 {
		procHeight = 5
	}

	procView := m.renderProcessTable(procHeight)
	connView := m.renderConnTable(procHeight)

	half := m.width / 2
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, procView, lipgloss.NewStyle().Width(half).Render(connView)))
	sb.WriteString("\n")

	// Help bar
	if m.showHelp {
		sb.WriteString(m.renderHelp())
	} else {
		sb.WriteString(helpStyle.Render("q quit  tab switch  s sort  / filter  f file-io  p pause  ? help"))
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// Render helpers
// ──────────────────────────────────────────────────────────────

func (m *AppModel) renderProcessTable(height int) string {
	half := m.width / 2
	colW := [8]int{6, 14, 10, 10, 10, 10, 5, 0}
	colW[7] = half - colW[0] - colW[1] - colW[2] - colW[3] - colW[4] - colW[5] - colW[6]
	if colW[7] < 4 {
		colW[7] = 4
	}

	header := headerStyle.Render(padR("PID", colW[0]) + padR("Name", colW[1]) +
		rxStyle.Render(padR("Net↓", colW[2])) + txStyle.Render(padR("Net↑", colW[3])) +
		fileStyle.Render(padR("FileR", colW[4])) + fileStyle.Render(padR("FileW", colW[5])) +
		padR("Conn", colW[6]))

	var rows []string
	rows = append(rows, header)

	start := 0
	if m.procCursor > height-3 {
		start = m.procCursor - (height - 3)
	}

	for i, p := range m.procs {
		if i < start {
			continue
		}
		if len(rows) >= height {
			break
		}
		row := fmt.Sprintf("%-*d%-*s", colW[0], p.PID, colW[1], truncate(p.Name(), colW[1]-1))
		row += padR(formatRate(p.Net.RxRate()), colW[2])
		row += padR(formatRate(p.Net.TxRate()), colW[3])
		row += padR(formatRate(p.File.RxRate()), colW[4])
		row += padR(formatRate(p.File.TxRate()), colW[5])
		row += padR(fmt.Sprintf("%d", p.ConnectionCount()), colW[6])

		if i == m.procCursor && m.activePanel == panelProcess {
			rows = append(rows, selectedStyle.Render(row))
		} else {
			rows = append(rows, row)
		}
	}

	return lipgloss.NewStyle().Width(half).Render(strings.Join(rows, "\n"))
}

func (m *AppModel) renderConnTable(height int) string {
	half := m.width / 2

	header := headerStyle.Render(padR("FD", 5) + padR("Class", 8) + padR("Proto", 6) +
		rxStyle.Render(padR("RX", 10)) + txStyle.Render(padR("TX", 10)) + "Endpoint")

	var rows []string
	rows = append(rows, header)

	start := 0
	if m.connCursor > height-3 {
		start = m.connCursor - (height - 3)
	}

	for i, c := range m.conns {
		if i < start {
			continue
		}
		if len(rows) >= height {
			break
		}
		info := c.Info()
		proto := "-"
		endpoint := "-"
		if info != nil {
			proto = info.Protocol
			endpoint = info.Remote
			if endpoint == "" {
				endpoint = info.Local
			}
		}
		row := fmt.Sprintf("%-5d%-8s%-6s%-10s%-10s%s",
			c.FD, c.Class.String(), proto,
			formatRate(c.IO.RxRate()), formatRate(c.IO.TxRate()),
			truncate(endpoint, half-39))

		if i == m.connCursor && m.activePanel == panelConn {
			rows = append(rows, selectedStyle.Render(row))
		} else {
			rows = append(rows, row)
		}
	}

	return strings.Join(rows, "\n")
}

func (m *AppModel) renderHelp() string {
	return helpStyle.Render(strings.Join([]string{
		"Keyboard shortcuts:",
		"  q / Ctrl+C  Quit",
		"  Tab         Switch between process/connection panel",
		"  ↑↓ / k j    Navigate",
		"  s           Cycle sort column",
		"  /           Filter by process name",
		"  f           Toggle file I/O display",
		"  p / F5      Pause/resume updates",
		"  ?           Toggle this help",
	}, "\n"))
}

// ──────────────────────────────────────────────────────────────
// State update helpers
// ──────────────────────────────────────────────────────────────

func (m *AppModel) refresh() {
	all := m.store.Processes()

	// Filter
	filter := strings.ToLower(m.filterStr)
	if filter != "" {
		var filtered []*model.Process
		for _, p := range all {
			if strings.Contains(strings.ToLower(p.Name()), filter) {
				filtered = append(filtered, p)
			}
		}
		all = filtered
	}

	m.procs = all
	m.sortProcs()

	// Clamp cursor
	if m.procCursor >= len(m.procs) && len(m.procs) > 0 {
		m.procCursor = len(m.procs) - 1
	}

	m.updateConnList()
}

func (m *AppModel) sortProcs() {
	procs := m.procs
	switch m.sortMode {
	case sortByNetRx:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].Net.RxRate() > procs[j].Net.RxRate()
		})
	case sortByNetTx:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].Net.TxRate() > procs[j].Net.TxRate()
		})
	case sortByFileRead:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].File.RxRate() > procs[j].File.RxRate()
		})
	case sortByFileWrite:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].File.TxRate() > procs[j].File.TxRate()
		})
	case sortByPID:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].PID < procs[j].PID
		})
	case sortByName:
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].Name() < procs[j].Name()
		})
	}
}

func (m *AppModel) updateConnList() {
	if len(m.procs) == 0 || m.procCursor >= len(m.procs) {
		m.conns = nil
		return
	}
	proc := m.procs[m.procCursor]
	var conns []*model.Connection
	proc.EachConnection(func(c *model.Connection) bool {
		conns = append(conns, c)
		return true
	})
	// Sort by tx+rx rate descending
	sort.Slice(conns, func(i, j int) bool {
		ri := conns[i].IO.RxRate() + conns[i].IO.TxRate()
		rj := conns[j].IO.RxRate() + conns[j].IO.TxRate()
		return ri > rj
	})
	m.conns = conns
	if m.connCursor >= len(m.conns) && len(m.conns) > 0 {
		m.connCursor = len(m.conns) - 1
	}
}

// ──────────────────────────────────────────────────────────────
// Formatting helpers
// ──────────────────────────────────────────────────────────────

// formatRate formats a bytes/second rate to a human-readable string.
func formatRate(bps float64) string {
	switch {
	case bps >= 1e9:
		return fmt.Sprintf("%.1fGB/s", bps/1e9)
	case bps >= 1e6:
		return fmt.Sprintf("%.1fMB/s", bps/1e6)
	case bps >= 1e3:
		return fmt.Sprintf("%.1fKB/s", bps/1e3)
	case bps > 0:
		return fmt.Sprintf("%.0fB/s", bps)
	default:
		return "-"
	}
}

// padR right-pads s to width w.
func padR(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

// truncate shortens s to max length n, adding "…" if truncated.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

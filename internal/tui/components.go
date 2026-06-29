package tui

import (
	"fmt"
	"strings"

	"github.com/Vrex123/pharos/internal/model"
	"github.com/charmbracelet/lipgloss"
)

const serverListWidth = 24

// View renders the full screen.
func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading pharos…"
	}

	if m.mode == modeAddForm || m.mode == modeEditForm {
		return focusedPanelStyle.Render(m.form.View())
	}

	serverPanel := m.renderServerList()
	statsPanel := m.renderStats()
	top := lipgloss.JoinHorizontal(lipgloss.Top, serverPanel, statsPanel)

	bottom := m.renderBottomPanel()

	footer := mutedStyle.Render("auto-refresh: "+m.autoRefreshLabel()) + "\n" + footerStyle.Render(footerHints(m.focus, m.bottomTab, m.width))
	if m.mode == modeConfirmDelete {
		if s, ok := m.selected(); ok {
			footer = errStyle.Render(fmt.Sprintf("Delete server %q? (y/n)", s.Name))
		}
	}
	if m.statusMsg != "" {
		footer = errStyle.Render(m.statusMsg) + "\n" + footer
	}

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom, footer)
}

func (m AppModel) renderServerList() string {
	style := panelStyle
	if m.focus == FocusServers {
		style = focusedPanelStyle
	}
	style = style.Width(serverListWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Servers") + "\n")

	if len(m.servers) == 0 {
		b.WriteString(mutedStyle.Render("no servers\npress 'a' to add"))
		return style.Render(b.String())
	}

	for i, s := range m.servers {
		marker, mStyle := m.statusMarker(s.Name)
		line := fmt.Sprintf("%s %s", mStyle.Render(marker), s.Name)
		if i == m.selectedServer {
			line = selectedItemStyle.Render(fmt.Sprintf(" %s %s ", marker, s.Name))
		}
		b.WriteString(line + "\n")
	}
	return style.Render(strings.TrimRight(b.String(), "\n"))
}

// statusMarker returns the indicator glyph and style for a server.
func (m AppModel) statusMarker(name string) (string, lipgloss.Style) {
	if m.loading[name] {
		return "…", unknownStyle
	}
	snap, ok := m.snapshots[name]
	if !ok {
		return "○", unknownStyle
	}
	if snap.Online {
		return "●", onlineStyle
	}
	return "×", offlineStyle
}

func (m AppModel) renderStats() string {
	style := panelStyle
	width := m.width - serverListWidth - 4
	if width < 20 {
		width = 20
	}
	style = style.Width(width)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Server Stats") + "\n")

	s, ok := m.selected()
	if !ok {
		return style.Render(b.String() + mutedStyle.Render("no server selected"))
	}

	snap, have := m.snapshots[s.Name]
	b.WriteString(fmt.Sprintf("host:   %s@%s:%d\n", s.User, s.Host, s.Port))

	switch {
	case m.loading[s.Name]:
		b.WriteString("status: " + mutedStyle.Render("refreshing…"))
	case !have:
		b.WriteString("status: " + mutedStyle.Render("not refreshed (press r)"))
	case snap.Online:
		st := snap.Stats
		b.WriteString("status: " + onlineStyle.Render("online") + "\n")
		b.WriteString("load avg: " + loadLine(st) + "\n")
		b.WriteString(fmt.Sprintf("ram:    %s / %s%s\n", humanBytes(st.MemUsed), humanBytes(st.MemTotal), percentSuffix(st.MemUsed, st.MemTotal)))
		b.WriteString(fmt.Sprintf("disk:   %s / %s (%s)", humanBytes(st.DiskUsed), humanBytes(st.DiskTotal), st.DiskPercent))
	default:
		b.WriteString("status: " + offlineStyle.Render("offline"))
	}

	if have && snap.Error != "" {
		b.WriteString("\n" + errStyle.Render("! "+truncate(snap.Error, width-2)))
	}

	return style.Render(b.String())
}

// maxProcessRows caps how many processes are listed. There is no vertical
// scrolling, and top is already CPU-sorted, so this shows the busiest ones.
const maxProcessRows = 20

// renderBottomPanel renders the focusable bottom panel, which is tabbed between
// the Docker Containers and Processes views (switched with [ / ]).
func (m AppModel) renderBottomPanel() string {
	style := panelStyle
	if m.focus == FocusContainers {
		style = focusedPanelStyle
	}
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	style = style.Width(width)

	var b strings.Builder
	b.WriteString(m.bottomTabBar() + "\n")
	if m.bottomTab == tabProcesses {
		b.WriteString(m.processesBody())
	} else {
		b.WriteString(m.containersBody())
	}
	return style.Render(strings.TrimRight(b.String(), "\n"))
}

// bottomTabBar renders the two tab labels, highlighting the active one.
func (m AppModel) bottomTabBar() string {
	containers, processes := "Docker Containers", "Processes"
	if m.bottomTab == tabProcesses {
		return mutedStyle.Render(containers) + "  " + titleStyle.Render(processes)
	}
	return titleStyle.Render(containers) + "  " + mutedStyle.Render(processes)
}

func (m AppModel) containersBody() string {
	s, ok := m.selected()
	if !ok {
		return mutedStyle.Render("no server selected")
	}
	snap, have := m.snapshots[s.Name]
	if !have {
		return mutedStyle.Render("not refreshed")
	}
	if !s.Docker {
		return mutedStyle.Render("docker disabled for this server")
	}
	if len(snap.Containers) == 0 {
		return mutedStyle.Render("no containers")
	}

	var b strings.Builder
	b.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-18s %-18s %-9s %-8s %-12s", "NAME", "IMAGE", "STATE", "CPU", "MEM")) + "\n")

	for i, c := range snap.Containers {
		cpu, mem := "-", "-"
		if st, ok := snap.DockerStats[c.Name]; ok {
			cpu, mem = st.CPUPerc, st.MemUsage
		} else if st, ok := snap.DockerStats[c.ID]; ok {
			cpu, mem = st.CPUPerc, st.MemUsage
		}
		row := fmt.Sprintf("%-18s %-18s %-9s %-8s %-12s",
			truncate(c.Name, 18), truncate(c.Image, 18), truncate(c.State, 9),
			truncate(cpu, 8), truncate(mem, 12))
		if m.focus == FocusContainers && i == m.selectedContainer {
			row = selectedItemStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

func (m AppModel) processesBody() string {
	s, ok := m.selected()
	if !ok {
		return mutedStyle.Render("no server selected")
	}
	snap, have := m.snapshots[s.Name]
	if !have {
		return mutedStyle.Render("not refreshed")
	}
	if len(snap.Processes) == 0 {
		return mutedStyle.Render("no processes")
	}

	// COMMAND fills whatever width remains after the fixed columns (which occupy
	// 43 cols incl. separators) and the panel's 2 cols of horizontal padding.
	cmdWidth := m.width - 4 - 2 - 43
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	var b strings.Builder
	b.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-7s %-10s %-6s %-6s %-9s %s", "PID", "USER", "%CPU", "%MEM", "TIME+", "COMMAND")) + "\n")

	for i, p := range snap.Processes {
		if i >= maxProcessRows {
			break
		}
		row := fmt.Sprintf("%-7s %-10s %-6s %-6s %-9s %s",
			truncate(p.PID, 7), truncate(p.User, 10), truncate(p.CPUPerc, 6),
			truncate(p.MemPerc, 6), truncate(p.Time, 9), truncate(p.Command, cmdWidth))
		if m.focus == FocusContainers && i == m.selectedProcess {
			row = selectedItemStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

// loadLine renders the load average. When the core count is known each of the
// 1/5/15-minute averages is annotated with its percentage of total CPU capacity
// (100% = all cores fully busy), followed by the core count for context. Without
// a known core count it falls back to the raw averages.
func loadLine(st model.ServerStats) string {
	if st.CPUCores <= 0 {
		return fmt.Sprintf("%.2f %.2f %.2f", st.Load1, st.Load5, st.Load15)
	}
	pct := func(load float64) int { return int(load/float64(st.CPUCores)*100 + 0.5) }
	cores := "cores"
	if st.CPUCores == 1 {
		cores = "core"
	}
	return fmt.Sprintf("%.2f (%d%%)  %.2f (%d%%)  %.2f (%d%%)  (%d %s)",
		st.Load1, pct(st.Load1), st.Load5, pct(st.Load5), st.Load15, pct(st.Load15),
		st.CPUCores, cores)
}

// autoRefreshLabel describes the current auto-refresh setting for the footer.
func (m AppModel) autoRefreshLabel() string {
	d := autoIntervals[m.autoIdx]
	if d == 0 {
		return "off"
	}
	return d.String()
}

// percentSuffix returns a " (NN%)" string for used/total, or "" when total is 0.
func percentSuffix(used, total uint64) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d%%)", used*100/total)
}

// humanBytes formats a byte count as a human-readable string.
func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

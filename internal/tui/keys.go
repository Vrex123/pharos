package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Tab           key.Binding
	SwitchTab     key.Binding
	Refresh       key.Binding
	RefreshAll    key.Binding
	Auto          key.Binding
	SSH           key.Binding
	ContainerExec key.Binding
	ContainerLogs key.Binding
	Add           key.Binding
	Edit          key.Binding
	Delete        key.Binding
	Quit          key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch focus"),
	),
	SwitchTab: key.NewBinding(
		key.WithKeys("[", "]"),
		key.WithHelp("[/]", "switch tab"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r", "enter"),
		key.WithHelp("r", "refresh"),
	),
	RefreshAll: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh all"),
	),
	Auto: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "auto-refresh"),
	),
	SSH: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "ssh"),
	),
	ContainerExec: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "exec"),
	),
	ContainerLogs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add server"),
	),
	Edit: key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("E", "edit server"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete server"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// serverHintKeys, containerHintKeys, and processHintKeys are the context-specific
// key groups shown when the respective panel (or bottom tab) is focused. The
// Processes tab is read-only, so it has no action keys of its own.
var (
	serverHintKeys    = []key.Binding{keys.Refresh, keys.RefreshAll, keys.Auto, keys.Add, keys.Edit, keys.Delete, keys.SSH}
	containerHintKeys = []key.Binding{keys.ContainerExec, keys.ContainerLogs}
	processHintKeys   = []key.Binding{}
)

const hintSep = " • "

// footerHints builds the key-hint block shown at the bottom for the currently
// focused panel: always-available navigation, the focus-specific actions, then
// quit. Hints are wrapped to width across as many lines as needed, never
// splitting an individual hint.
func footerHints(focus FocusArea, tab bottomTab, width int) string {
	bindings := []key.Binding{keys.Up, keys.Down, keys.Tab}
	switch focus {
	case FocusServers:
		bindings = append(bindings, serverHintKeys...)
	case FocusContainers:
		bindings = append(bindings, keys.SwitchTab)
		if tab == tabProcesses {
			bindings = append(bindings, processHintKeys...)
		} else {
			bindings = append(bindings, containerHintKeys...)
		}
	}
	bindings = append(bindings, keys.Quit)

	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return wrapHints(parts, width)
}

// wrapHints joins parts with hintSep, wrapping onto a new line whenever the next
// part would exceed width. A non-positive width disables wrapping.
func wrapHints(parts []string, width int) string {
	if len(parts) == 0 {
		return ""
	}
	sepW := lipgloss.Width(hintSep)
	var b strings.Builder
	lineW := 0
	for i, p := range parts {
		pw := lipgloss.Width(p)
		switch {
		case i == 0:
			b.WriteString(p)
			lineW = pw
		case width > 0 && lineW+sepW+pw > width:
			b.WriteByte('\n')
			b.WriteString(p)
			lineW = pw
		default:
			b.WriteString(hintSep)
			b.WriteString(p)
			lineW += sepW + pw
		}
	}
	return b.String()
}

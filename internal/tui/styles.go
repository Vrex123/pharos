package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.Color("12")  // blue
	colorOnline  = lipgloss.Color("10")  // green
	colorOffline = lipgloss.Color("9")   // red
	colorUnknown = lipgloss.Color("244") // gray
	colorMuted   = lipgloss.Color("244")
	colorErr     = lipgloss.Color("9")

	// panelStyle is the default (unfocused) bordered panel.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorUnknown).
			Padding(0, 1)

	// focusedPanelStyle highlights the panel that currently has focus.
	focusedPanelStyle = panelStyle.
				BorderForeground(colorAccent)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(colorAccent)

	onlineStyle  = lipgloss.NewStyle().Foreground(colorOnline)
	offlineStyle = lipgloss.NewStyle().Foreground(colorOffline)
	unknownStyle = lipgloss.NewStyle().Foreground(colorUnknown)

	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	errStyle   = lipgloss.NewStyle().Foreground(colorErr)

	footerStyle = lipgloss.NewStyle().Foreground(colorMuted)

	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorMuted)
)

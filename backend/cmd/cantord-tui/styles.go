package main

import "github.com/charmbracelet/lipgloss"

var (
	accent = lipgloss.AdaptiveColor{Light: "#7048e8", Dark: "#a78bfa"}
	subtle = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"}
	good   = lipgloss.AdaptiveColor{Light: "#2b8a3e", Dark: "#69db7c"}
	bad    = lipgloss.AdaptiveColor{Light: "#c92a2a", Dark: "#ff8787"}

	listTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent)

	footerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle).
			Padding(0, 1)

	footerLabelStyle = lipgloss.NewStyle().Foreground(subtle)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			Padding(0, 1)

	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(subtle)

	connectedStyle    = lipgloss.NewStyle().Foreground(good)
	disconnectedStyle = lipgloss.NewStyle().Foreground(bad).Bold(true)

	errorStyle = lipgloss.NewStyle().Foreground(bad)

	searchSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)

	promptBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2)
)

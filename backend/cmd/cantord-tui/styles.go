package main

import "github.com/charmbracelet/lipgloss"

var (
	accent = lipgloss.AdaptiveColor{Light: "#7048e8", Dark: "#a78bfa"}
	subtle = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"}
	good   = lipgloss.AdaptiveColor{Light: "#2b8a3e", Dark: "#69db7c"}
	bad    = lipgloss.AdaptiveColor{Light: "#c92a2a", Dark: "#ff8787"}

	tabBarStyle = lipgloss.NewStyle().Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(subtle).
				Padding(0, 1)

	listTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent)

	footerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle).
			Padding(0, 1)

	footerLabelStyle = lipgloss.NewStyle().Foreground(subtle)

	connectedStyle    = lipgloss.NewStyle().Foreground(good)
	disconnectedStyle = lipgloss.NewStyle().Foreground(bad).Bold(true)

	errorStyle = lipgloss.NewStyle().Foreground(bad)

	promptBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2)
)

func renderTabBar(titles []string, active int) string {
	parts := make([]string, len(titles))
	for i, t := range titles {
		style := inactiveTabStyle
		if i == active {
			style = activeTabStyle
		}
		parts[i] = style.Render(t)
	}
	return tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

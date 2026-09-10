package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// helpEntry is one row of the "?" help screen: a key (or chord of
// equivalent keys) and what it does.
type helpEntry struct {
	key  string
	desc string
}

// fromBinding pulls key/desc straight off a key.Binding's own WithHelp
// metadata, so the help screen can't drift out of sync with keys.go.
func fromBinding(b key.Binding) helpEntry {
	h := b.Help()
	return helpEntry{key: h.Key, desc: h.Desc}
}

// helpSections lists every shortcut in the app. The list-navigation rows
// (cursor movement, paging, filtering, go-to-start/end) live on list.Model
// itself rather than in the global keys struct, so those are spelled out
// by hand here to match what's actually wired up in newListWithDelegate.
var helpSections = []struct {
	title   string
	entries []helpEntry
}{
	{"Navigate", []helpEntry{
		fromBinding(keys.into),
		fromBinding(keys.back),
		fromBinding(keys.selectItem),
		{"↑/k  ↓/j", "move cursor"},
		{"g/home  G/end", "jump to top/bottom"},
		{"H/←/PgUp/^u", "prev page"},
		{"L/→/PgDn/^d", "next page"},
		{"/", "filter"},
		{"esc", "clear filter"},
	}},
	{"Sections", []helpEntry{
		fromBinding(keys.tab1),
		fromBinding(keys.tab2),
		fromBinding(keys.tab3),
		fromBinding(keys.tab4),
		fromBinding(keys.tab5),
		fromBinding(keys.nextTab),
		fromBinding(keys.prevTab),
	}},
	{"Playback", []helpEntry{
		fromBinding(keys.playPause),
		fromBinding(keys.next),
		fromBinding(keys.prev),
		fromBinding(keys.volUp),
		fromBinding(keys.volDown),
		fromBinding(keys.mute),
		fromBinding(keys.shuffle),
		fromBinding(keys.repeat),
	}},
	{"Library", []helpEntry{
		fromBinding(keys.enqueue),
		fromBinding(keys.addToPlaylist),
		fromBinding(keys.remove),
		fromBinding(keys.moveDown),
		fromBinding(keys.moveUp),
		fromBinding(keys.toggleCover),
	}},
	{"App", []helpEntry{
		fromBinding(keys.help),
		fromBinding(keys.quit),
		{"ctrl+c", "force quit"},
	}},
}

// renderHelp centers a box listing every keyboard shortcut, grouped into
// helpSections, over the full width/height of the terminal.
func renderHelp(width, height int) string {
	keyWidth := 0
	for _, sec := range helpSections {
		for _, e := range sec.entries {
			if w := lipgloss.Width(e.key); w > keyWidth {
				keyWidth = w
			}
		}
	}
	keyStyle := lipgloss.NewStyle().Foreground(accent).Bold(true).Width(keyWidth)

	var lines []string
	lines = append(lines, listTitleStyle.Render("Keyboard Shortcuts"), "")
	for i, sec := range helpSections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, footerLabelStyle.Render(strings.ToUpper(sec.title)))
		for _, e := range sec.entries {
			lines = append(lines, "  "+keyStyle.Render(e.key)+"  "+e.desc)
		}
	}

	box := promptBoxStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

package main

import "github.com/charmbracelet/bubbles/key"

// globalKeys are the app-wide vim-style bindings handled by the root model
// before (or instead of) forwarding a key to the active screen's list. They
// double as the source for the "?" full-help view via list.Model's
// AdditionalFullHelpKeys hook.
type globalKeyMap struct {
	tab1, tab2, tab3, tab4, tab5 key.Binding
	nextTab                      key.Binding
	prevTab                      key.Binding
	back                               key.Binding
	into                               key.Binding
	quit                               key.Binding
	forceQuit                          key.Binding
	selectItem                         key.Binding
	playPause                          key.Binding
	next                               key.Binding
	prev                               key.Binding
	volUp                              key.Binding
	volDown                            key.Binding
	mute                               key.Binding
	shuffle                            key.Binding
	repeat                             key.Binding
	enqueue                            key.Binding
	addToPlaylist                      key.Binding
	remove                             key.Binding
	moveDown                           key.Binding
	moveUp                             key.Binding
}

var keys = globalKeyMap{
	tab1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "artists")),
	tab2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "albums")),
	tab3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "genres")),
	tab4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "queue")),
	tab5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "playlists")),

	nextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next menu")),
	prevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev menu")),

	back:      key.NewBinding(key.WithKeys("esc", "backspace", "h"), key.WithHelp("h", "back")),
	into:      key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "into")),
	quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	forceQuit: key.NewBinding(key.WithKeys("ctrl+c")),

	selectItem: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),

	playPause: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
	next:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next track")),
	prev:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev track")),
	volUp:     key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "vol up")),
	volDown:   key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "vol down")),
	mute:      key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
	shuffle:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shuffle")),
	repeat:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),

	enqueue:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add to queue")),
	addToPlaylist: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "add to playlist")),
	remove:        key.NewBinding(key.WithKeys("d"), key.WithHelp("dd", "remove")),
	moveDown:      key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "move down")),
	moveUp:        key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "move up")),
}

// ShortHelp/FullHelp let this double as an AdditionalFullHelpKeys source on
// every list, so "?" shows navigation, transport, and per-screen actions
// together in one place.
func (k globalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.into, keys.back, keys.selectItem, keys.playPause, keys.quit}
}

func (k globalKeyMap) FullHelp() []key.Binding {
	return []key.Binding{
		keys.tab1, keys.tab2, keys.tab3, keys.tab4, keys.tab5,
		keys.nextTab, keys.prevTab,
		keys.into, keys.back, keys.selectItem,
		keys.playPause, keys.next, keys.prev, keys.volUp, keys.volDown, keys.mute, keys.shuffle, keys.repeat,
		keys.enqueue, keys.addToPlaylist, keys.remove, keys.moveDown, keys.moveUp,
		keys.quit,
	}
}

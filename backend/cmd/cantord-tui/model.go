package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var tabTitles = []string{"1 Artists", "2 Albums", "3 Genres", "4 Queue", "5 Playlists"}

const (
	tabArtists = iota
	tabAlbums
	tabGenres
	tabQueue
	tabPlaylists
)

// addPrompt is the small modal shown for "A" (add to playlist): it replaces
// the whole screen while active rather than trying to composite a floating
// box over the list, which keeps the rendering simple and unambiguous.
type addPrompt struct {
	trackID    string
	trackLabel string
	input      textinput.Model
}

type Model struct {
	client  *Client
	events  chan tea.Msg
	initCmd tea.Cmd

	width, height int

	tabs      [5][]screen
	activeTab int

	status    Status
	connected bool

	prompt *addPrompt

	pendingD bool
	ddGen    int
}

func newModel(client *Client, events chan tea.Msg) Model {
	m := Model{client: client, events: events, connected: true}
	s, cmd := newArtistsScreen(client)
	m.tabs[tabArtists] = []screen{s}
	m.initCmd = tea.Batch(cmd, waitForMsg(events))
	return m
}

func waitForMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

type ddClearMsg struct{ gen int }

func ddTimeout(gen int) tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg { return ddClearMsg{gen: gen} })
}

func (m Model) Init() tea.Cmd { return m.initCmd }

// --- navigation helpers (pointer receiver so callers mutate the Model's
// local copy in place inside Update) ---

func (m *Model) currentScreen() *screen {
	stack := m.tabs[m.activeTab]
	if len(stack) == 0 {
		return nil
	}
	return &stack[len(stack)-1]
}

func (m *Model) footerHeight() int { return 3 } // border-top + 2 content lines

func (m *Model) contentSize() (int, int) {
	h := m.height - m.footerHeight() - 1 // -1 for the tab bar
	if h < 3 {
		h = 3
	}
	return m.width, h
}

func (m *Model) resizeScreen(s *screen) {
	if s == nil {
		return
	}
	w, h := m.contentSize()
	s.list.SetSize(w, h)
}

func (m *Model) resizeAll() {
	for t := range m.tabs {
		for i := range m.tabs[t] {
			m.resizeScreen(&m.tabs[t][i])
		}
	}
}

// breadcrumbTitle joins the leaf titles of the current stack with the new
// screen's own title, so the list's title bar always shows the full path
// (e.g. "Artists › Pendulum › Immersion") instead of just the leaf name —
// the only way to tell where you are once you've drilled in more than one
// level.
func (m *Model) breadcrumbTitle(leaf string) string {
	stack := m.tabs[m.activeTab]
	crumbs := make([]string, 0, len(stack)+1)
	for _, s := range stack {
		crumbs = append(crumbs, s.title)
	}
	crumbs = append(crumbs, leaf)
	return strings.Join(crumbs, " › ")
}

func (m *Model) push(s screen) {
	s.list.Title = m.breadcrumbTitle(s.title)
	m.resizeScreen(&s)
	m.tabs[m.activeTab] = append(m.tabs[m.activeTab], s)
}

func (m *Model) goBack() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	if cur.list.FilterState() != list.Unfiltered {
		var cmd tea.Cmd
		cur.list, cmd = cur.list.Update(tea.KeyMsg{Type: tea.KeyEsc})
		return cmd
	}
	if len(m.tabs[m.activeTab]) > 1 {
		m.tabs[m.activeTab] = m.tabs[m.activeTab][:len(m.tabs[m.activeTab])-1]
	}
	return nil
}

func (m *Model) switchTab(i int) tea.Cmd {
	m.activeTab = i
	if len(m.tabs[i]) > 0 {
		m.resizeScreen(m.currentScreen())
		return nil
	}
	var s screen
	var cmd tea.Cmd
	switch i {
	case tabArtists:
		s, cmd = newArtistsScreen(m.client)
	case tabAlbums:
		s, cmd = newAlbumsScreen(m.client)
	case tabGenres:
		s, cmd = newGenresScreen(m.client)
	case tabQueue:
		s, cmd = newQueueScreen(m.client)
	case tabPlaylists:
		s, cmd = newPlaylistsScreen(m.client)
	}
	m.resizeScreen(&s)
	m.tabs[i] = []screen{s}
	return cmd
}

// findScreen locates a screen by id anywhere across every tab's stack, so a
// background fetch can land correctly even if the user has since navigated
// elsewhere.
func (m *Model) findScreen(id int) *screen {
	for t := range m.tabs {
		for i := range m.tabs[t] {
			if m.tabs[t][i].id == id {
				return &m.tabs[t][i]
			}
		}
	}
	return nil
}

func (m *Model) queueScreen() *screen {
	if len(m.tabs[tabQueue]) == 0 {
		return nil
	}
	return &m.tabs[tabQueue][0]
}

// --- item-kind-driven actions, shared across every screen that happens to
// contain that kind of item (search results, album tracks, genre tracks,
// and playlist tracks are all just lists of track items, for instance). ---

// drillInto pushes a new screen for the selected item's children ("l" — the
// tree-navigation counterpart to goBack's "h"). Tracks have no children, so
// it's a no-op on a track; playing one is enter's job (playCurrent).
func (m *Model) drillInto() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	switch it.kind {
	case itemArtist:
		s, cmd := newAlbumsByArtistScreen(m.client, it.id)
		m.push(s)
		return cmd
	case itemGenre:
		s, cmd := newGenreTracksScreen(m.client, it.id)
		m.push(s)
		return cmd
	case itemPlaylist:
		s, cmd := newPlaylistTracksScreen(m.client, it.id)
		m.push(s)
		return cmd
	case itemAlbum:
		s, cmd := newAlbumTracksScreen(m.client, *it.album)
		m.push(s)
		return cmd
	}
	return nil
}

// playCurrent plays the selected track. Enter is reserved for this alone —
// navigating into a folder-like item is drillInto's job ("l").
func (m *Model) playCurrent() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok || it.kind != itemTrack {
		return nil
	}
	if cur.kind == screenQueue {
		return cmdPlayQueueIndex(m.client, cur.list.Index())
	}
	return cmdPlayNow(m.client, it.id)
}

func (m *Model) enqueueCurrent() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	switch it.kind {
	case itemTrack:
		return cmdEnqueueTrack(m.client, it.id)
	case itemAlbum:
		return cmdEnqueueAlbum(m.client, it.id, it.title)
	case itemPlaylist:
		return cmdEnqueuePlaylist(m.client, it.id)
	}
	return nil
}

func (m *Model) startAddToPlaylist() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok || it.kind != itemTrack {
		return nil
	}
	ti := textinput.New()
	ti.Placeholder = "playlist name"
	ti.CharLimit = 64
	ti.Width = 40
	ti.Focus()
	m.prompt = &addPrompt{trackID: it.id, trackLabel: it.title, input: ti}
	return textinput.Blink
}

func (m *Model) removeCurrent() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	switch cur.kind {
	case screenQueue:
		return cmdRemoveQueueIndex(m.client, cur.list.Index())
	case screenPlaylistTracks:
		return cmdRemoveFromPlaylist(m.client, cur.ctx, it.id)
	}
	return nil
}

func (m *Model) moveCurrent(dir int) tea.Cmd {
	cur := m.currentScreen()
	if cur == nil || cur.kind != screenQueue {
		return nil
	}
	from := cur.list.Index()
	to := from + dir
	if to < 0 || to >= len(cur.list.Items()) {
		return nil
	}
	return cmdMoveQueue(m.client, from, to)
}

// --- Update / View ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeAll()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case sseMsg:
		cmd := m.handleSSE(sseEvent(msg))
		return m, tea.Batch(cmd, waitForMsg(m.events))

	case connLostMsg:
		m.connected = false
		return m, waitForMsg(m.events)

	case itemsLoadedMsg:
		if s := m.findScreen(msg.screenID); s != nil {
			cmd := setItems(&s.list, msg.items)
			if msg.err != nil {
				return m, s.list.NewStatusMessage(errorStyle.Render(msg.err.Error()))
			}
			return m, cmd
		}
		return m, nil

	case albumsPageLoadedMsg:
		if s := m.findScreen(msg.screenID); s != nil {
			items := msg.items
			if msg.append {
				existing := s.list.Items()
				merged := make([]item, 0, len(existing)+len(items))
				for _, li := range existing {
					merged = append(merged, li.(item))
				}
				merged = append(merged, items...)
				items = merged
			}
			s.nextCursor = msg.nextCursor
			cmd := setItems(&s.list, items)
			if msg.err != nil {
				return m, s.list.NewStatusMessage(errorStyle.Render(msg.err.Error()))
			}
			return m, cmd
		}
		return m, nil

	case queueLoadedMsg:
		if s := m.findScreen(msg.screenID); s != nil {
			s.playingIndex = msg.playingIndex
			cmd := setItems(&s.list, msg.items)
			if msg.err != nil {
				return m, s.list.NewStatusMessage(errorStyle.Render(msg.err.Error()))
			}
			return m, cmd
		}
		return m, nil

	case actionResultMsg:
		var cmds []tea.Cmd
		if cur := m.currentScreen(); cur != nil {
			if msg.err != nil {
				cmds = append(cmds, cur.list.NewStatusMessage(errorStyle.Render(msg.err.Error())))
			} else if msg.text != "" {
				cmds = append(cmds, cur.list.NewStatusMessage(msg.text))
			}
		}
		if msg.refreshQueue && msg.err == nil {
			if qs := m.queueScreen(); qs != nil {
				cmds = append(cmds, loadQueue(m.client, qs.id))
			}
		}
		return m, tea.Batch(cmds...)

	case ddClearMsg:
		if msg.gen == m.ddGen {
			m.pendingD = false
		}
		return m, nil
	}

	// Anything else (spinner ticks, textinput blink, filter-match results,
	// etc.) belongs to whichever sub-component is currently live.
	if m.prompt != nil {
		var cmd tea.Cmd
		m.prompt.input, cmd = m.prompt.input.Update(msg)
		return m, cmd
	}
	if cur := m.currentScreen(); cur != nil {
		var cmd tea.Cmd
		cur.list, cmd = cur.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleSSE(ev sseEvent) tea.Cmd {
	switch ev.name {
	case "status":
		var st Status
		if json.Unmarshal([]byte(ev.data), &st) == nil {
			m.status = st
		}
		m.connected = true
	case "queue_changed":
		var tracks []Track
		if json.Unmarshal([]byte(ev.data), &tracks) == nil {
			if qs := m.queueScreen(); qs != nil {
				items := make([]item, len(tracks))
				for i, t := range tracks {
					items[i] = trackItem(t)
				}
				return setItems(&qs.list, items)
			}
		}
	}
	return nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.prompt != nil {
		switch msg.String() {
		case "esc":
			m.prompt = nil
			return m, nil
		case "enter":
			name := m.prompt.input.Value()
			trackID := m.prompt.trackID
			m.prompt = nil
			if name == "" {
				return m, nil
			}
			return m, cmdAddToPlaylist(m.client, name, trackID)
		default:
			var cmd tea.Cmd
			m.prompt.input, cmd = m.prompt.input.Update(msg)
			return m, cmd
		}
	}

	cur := m.currentScreen()

	if cur != nil && cur.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		cur.list, cmd = cur.list.Update(msg)
		return m, cmd
	}

	wasPendingD := m.pendingD
	m.pendingD = false
	if wasPendingD && msg.String() == "d" {
		return m, m.removeCurrent()
	}

	switch {
	case key.Matches(msg, keys.tab1):
		return m, m.switchTab(tabArtists)
	case key.Matches(msg, keys.tab2):
		return m, m.switchTab(tabAlbums)
	case key.Matches(msg, keys.tab3):
		return m, m.switchTab(tabGenres)
	case key.Matches(msg, keys.tab4):
		return m, m.switchTab(tabQueue)
	case key.Matches(msg, keys.tab5):
		return m, m.switchTab(tabPlaylists)

	case key.Matches(msg, keys.nextTab):
		return m, m.switchTab((m.activeTab + 1) % len(tabTitles))
	case key.Matches(msg, keys.prevTab):
		return m, m.switchTab((m.activeTab - 1 + len(tabTitles)) % len(tabTitles))

	case key.Matches(msg, keys.back):
		return m, m.goBack()
	case key.Matches(msg, keys.into):
		return m, m.drillInto()

	case key.Matches(msg, keys.quit):
		return m, tea.Quit

	case key.Matches(msg, keys.selectItem):
		return m, m.playCurrent()

	case key.Matches(msg, keys.playPause):
		return m, cmdTogglePlayPause(m.client, m.status.State == "playing")
	case key.Matches(msg, keys.next):
		return m, cmdNext(m.client)
	case key.Matches(msg, keys.prev):
		return m, cmdPrev(m.client)
	case key.Matches(msg, keys.volUp):
		return m, cmdVolume(m.client, m.status.Volume, 5)
	case key.Matches(msg, keys.volDown):
		return m, cmdVolume(m.client, m.status.Volume, -5)
	case key.Matches(msg, keys.mute):
		return m, cmdToggleMute(m.client, m.status.Muted)
	case key.Matches(msg, keys.shuffle):
		return m, cmdToggleShuffle(m.client, m.status.Shuffle)
	case key.Matches(msg, keys.repeat):
		return m, cmdCycleRepeat(m.client, m.status.Repeat)

	case key.Matches(msg, keys.enqueue):
		return m, m.enqueueCurrent()
	case key.Matches(msg, keys.addToPlaylist):
		return m, m.startAddToPlaylist()
	case key.Matches(msg, keys.remove):
		m.pendingD = true
		m.ddGen++
		return m, ddTimeout(m.ddGen)
	case key.Matches(msg, keys.moveDown):
		return m, m.moveCurrent(1)
	case key.Matches(msg, keys.moveUp):
		return m, m.moveCurrent(-1)
	}

	if cur != nil {
		var cmd tea.Cmd
		cur.list, cmd = cur.list.Update(msg)
		if cur.kind == screenAlbums && cur.nextCursor != "" {
			if idx := cur.list.Index(); idx >= len(cur.list.Items())-5 && !cur.loadingMore {
				cur.loadingMore = true
				return m, tea.Batch(cmd, loadAlbumsPage(m.client, cur.id, cur.nextCursor))
			}
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "starting cantord-tui…"
	}

	if m.prompt != nil {
		box := promptBoxStyle.Render(fmt.Sprintf("Add to playlist\n\n%s\n\n%s", m.prompt.trackLabel, m.prompt.input.View()))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	tabBar := renderTabBar(tabTitles, m.activeTab)

	cur := m.tabs[m.activeTab]
	var body string
	if len(cur) == 0 {
		body = "loading…"
	} else {
		s := cur[len(cur)-1]
		body = s.list.View()
	}

	footer := renderFooter(m.status, m.width, m.connected)
	return tabBar + "\n" + body + "\n" + footer
}

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tabArtists = iota
	tabAlbums
	tabGenres
	tabQueue
	tabPlaylists
	numTabs
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

	tabs      [numTabs][]screen
	activeTab int

	status    Status
	connected bool

	prompt *addPrompt

	pendingD bool
	ddGen    int

	// artCache/artFetching are keyed by art hash and shared across every
	// Model copy bubbletea hands back and forth (maps, like slices, carry
	// their backing storage by reference). playingIndex is a pointer for
	// the same reason: the Queue delegate is handed it once at screen
	// construction and needs to keep seeing updates made to it long after.
	artCache     map[string]image.Image
	artFetching  map[string]bool
	playingIndex *int

	// showCoverArt is the "c" toggle for the Queue screen's art pane.
	showCoverArt bool

	// showHelp is the "?" full-screen shortcut reference.
	showHelp bool
}

func newModel(client *Client, events chan tea.Msg) Model {
	playingIndex := -1
	m := Model{
		client:       client,
		events:       events,
		connected:    true,
		artCache:     map[string]image.Image{},
		artFetching:  map[string]bool{},
		playingIndex: &playingIndex,
		showCoverArt: true,
	}
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
	h := m.height - m.footerHeight()
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
	if s.kind == screenQueue && m.showCoverArt {
		w, _ = queueSplit(w, h)
	}
	s.list.SetSize(w, h)
}

// queueSplit divides the Queue screen's content area between the track list
// and the now-playing art pane. The list always gets queueMinListWidth
// first; only whatever's left over — up to queueArtMaxWidth — goes to art,
// and art disappears entirely once there isn't enough room left for it to
// read as a picture rather than a smear of blocks.
const (
	queueMinListWidth = 40
	queueArtGap       = 2
	queueArtMinWidth  = 10
	queueArtMaxWidth  = 40
)

func queueSplit(total, height int) (listW, artW int) {
	avail := total - queueMinListWidth - queueArtGap
	if avail < queueArtMinWidth {
		return total, 0
	}
	artW = min(avail, queueArtMaxWidth)
	return total - artW - queueArtGap, artW
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
		s, cmd = newQueueScreen(m.client, m.playingIndex)
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

// playCurrent plays the selected item. Enter is reserved for this alone —
// navigating into a folder-like item is drillInto's job ("l"). On an album
// it replaces the queue with the whole album; on a track outside the Queue
// screen it replaces the queue with that track plus everything listed
// below it. Within the Queue screen itself, a track just jumps playback to
// that position rather than rebuilding the queue from its own tail.
func (m *Model) playCurrent() tea.Cmd {
	cur := m.currentScreen()
	if cur == nil {
		return nil
	}
	it, ok := cur.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	switch it.kind {
	case itemAlbum:
		return cmdPlayAlbum(m.client, it.id, it.title)
	case itemTrack:
		if cur.kind == screenQueue {
			return cmdPlayQueueIndex(m.client, cur.list.Index())
		}
		items := cur.list.Items()
		ids := make([]string, 0, len(items)-cur.list.Index())
		for _, li := range items[cur.list.Index():] {
			if ti, ok := li.(item); ok && ti.kind == itemTrack {
				ids = append(ids, ti.id)
			}
		}
		return cmdPlayTracksFrom(m.client, ids, it.title)
	}
	return nil
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
			// The API paginates albums ordered by artist then name; re-sort
			// by album name alone since that's the order the album list
			// shows.
			sort.Slice(items, func(i, j int) bool {
				return strings.ToLower(items[i].title) < strings.ToLower(items[j].title)
			})
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

	case artLoadedMsg:
		delete(m.artFetching, msg.hash)
		if msg.err == nil && msg.img != nil {
			m.artCache[msg.hash] = msg.img
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
		return m.ensureArtLoaded()
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

// ensureArtLoaded keeps playingIndex in sync with the current status and,
// if the now-playing track's art isn't cached (or already in flight),
// kicks off a fetch for it.
func (m *Model) ensureArtLoaded() tea.Cmd {
	*m.playingIndex = -1
	if m.status.State == "stopped" || m.status.Track == nil {
		return nil
	}
	if m.status.QueueIndex >= 0 {
		*m.playingIndex = m.status.QueueIndex
	}

	hash := m.status.Track.ArtHash
	if hash == "" || m.artCache[hash] != nil || m.artFetching[hash] {
		return nil
	}
	m.artFetching[hash] = true
	return loadArt(m.client, hash)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.forceQuit) {
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

	if m.showHelp {
		if key.Matches(msg, keys.help) || key.Matches(msg, keys.back) || key.Matches(msg, keys.quit) {
			m.showHelp = false
		}
		return m, nil
	}

	cur := m.currentScreen()

	if cur != nil && cur.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		cur.list, cmd = cur.list.Update(msg)
		return m, cmd
	}

	wasPendingD := m.pendingD
	m.pendingD = false
	if wasPendingD && key.Matches(msg, keys.remove) {
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
		return m, m.switchTab((m.activeTab + 1) % numTabs)
	case key.Matches(msg, keys.prevTab):
		return m, m.switchTab((m.activeTab - 1 + numTabs) % numTabs)

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

	case key.Matches(msg, keys.toggleCover):
		m.showCoverArt = !m.showCoverArt
		m.resizeScreen(m.queueScreen())
		return m, nil

	case key.Matches(msg, keys.help):
		m.showHelp = true
		return m, nil
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

	if m.showHelp {
		return renderHelp(m.width, m.height)
	}

	cur := m.tabs[m.activeTab]
	var body string
	if len(cur) == 0 {
		body = "loading…"
	} else {
		s := cur[len(cur)-1]
		body = s.list.View()
		if s.kind == screenQueue && m.showCoverArt {
			_, h := m.contentSize()
			if _, artW := queueSplit(m.width, h); artW > 0 {
				gap := lipgloss.NewStyle().Width(queueArtGap).Height(h).Render("")
				body = lipgloss.JoinHorizontal(lipgloss.Top, body, gap, m.renderQueueArt(artW, h))
			}
		}
	}

	footer := renderFooter(m.status, m.width, m.connected)
	return body + "\n" + footer
}

// renderQueueArt renders the now-playing track's cover art (if it's been
// fetched) centered inside a w x h box; it returns a blank box of that size
// otherwise, so the layout doesn't jump around while art is loading or
// nothing is playing.
func (m Model) renderQueueArt(w, h int) string {
	var content string
	if m.status.Track != nil && m.status.State != "stopped" {
		if img := m.artCache[m.status.Track.ArtHash]; img != nil {
			content = renderArt(img, w, h)
		}
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

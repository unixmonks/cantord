package main

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type screenKind int

const (
	screenArtists screenKind = iota
	screenAlbumsByArtist
	screenAlbums
	screenAlbumTracks
	screenGenres
	screenGenreTracks
	screenQueue
	screenPlaylists
	screenPlaylistTracks
)

// screen is one entry in a tab's navigation stack. Every screen owns its
// own list.Model so cursor position, scroll offset, and any active filter
// are preserved when the user drills in and backs out.
type screen struct {
	id    int
	kind  screenKind
	title string
	list  list.Model
	ctx   string // artist / genre / playlist name this screen was opened for

	// screenAlbums pagination (the only list that isn't fetched in full).
	nextCursor  string
	loadingMore bool

	// screenQueue: index of the currently-playing entry, for highlighting.
	playingIndex int
}

var nextScreenID int

func newScreenID() int {
	nextScreenID++
	return nextScreenID
}

func newList(title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	// Indent the description (year/artist/track-count, or artist/duration/
	// format) a couple columns further in than the title above it, so the
	// two lines read as a clear title/subtitle pair rather than two flush
	// left edges.
	const descIndent = 2
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.PaddingLeft(delegate.Styles.NormalDesc.GetPaddingLeft() + descIndent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.PaddingLeft(delegate.Styles.SelectedDesc.GetPaddingLeft() + descIndent)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.PaddingLeft(delegate.Styles.DimmedDesc.GetPaddingLeft() + descIndent)
	return newListWithDelegate(title, delegate)
}

// newCompactList is for screens whose items carry no description (Artists,
// Genres, Playlists) — a single-line-per-item delegate instead of the
// default's title+description pair, so the list isn't full of empty second
// lines.
func newCompactList(title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	return newListWithDelegate(title, delegate)
}

func newListWithDelegate(title string, delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.Title = title
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = keys.ShortHelp
	l.AdditionalFullHelpKeys = keys.FullHelp
	l.Styles.Title = listTitleStyle

	// Free "d"/"u"/"f"/"b" from the built-in pager so they're available for
	// our own bindings (dd to remove). Lowercase h/l are reserved globally
	// for tree navigation (back/into), so paging moves to uppercase H/L
	// plus the arrow/page keys.
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("left", "H", "pgup"), key.WithHelp("H/pgup", "prev page"))
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("right", "L", "pgdown"), key.WithHelp("L/pgdn", "next page"))
	// We handle quitting ourselves so "q"/"esc" don't fall through to the
	// list's own (would-be) tea.Quit.
	l.KeyMap.Quit = key.Binding{}
	l.KeyMap.ForceQuit = key.Binding{}
	return l
}

func setItems(l *list.Model, items []item) tea.Cmd {
	list := make([]list.Item, len(items))
	for i, it := range items {
		list[i] = it
	}
	cmd := l.SetItems(list)
	// bubbles' list.SetItems recomputes how much height the title/status/
	// pagination/help rows need, but it measures the pagination row's height
	// using the *previous* item count's TotalPages (it's only updated at the
	// end of that same call) — so going from "no items yet" to a real count
	// crossing the single-page threshold reserves the wrong height and the
	// list renders one line taller than its size, pushing everything below
	// it down and off screen. Re-applying the current size forces another
	// pass that measures against the now-correct TotalPages.
	l.SetSize(l.Width(), l.Height())
	return cmd
}

// --- screen constructors; each returns the screen plus the tea.Cmd that
// loads its data. ---

func newArtistsScreen(c *Client) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenArtists, title: "Artists", list: newCompactList("Artists")}
	id := s.id
	return s, func() tea.Msg {
		artists, err := c.Artists()
		items := make([]item, len(artists))
		for i, a := range artists {
			items[i] = artistItem(a)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newAlbumsByArtistScreen(c *Client, artist string) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenAlbumsByArtist, title: artist, ctx: artist, list: newList(artist)}
	id := s.id
	return s, func() tea.Msg {
		albums, err := c.AlbumsByArtist(artist)
		items := make([]item, len(albums))
		for i, a := range albums {
			items[i] = albumItem(a)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newAlbumsScreen(c *Client) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenAlbums, title: "Albums", list: newList("Albums")}
	return s, loadAlbumsPage(c, s.id, "")
}

func loadAlbumsPage(c *Client, id int, cursor string) tea.Cmd {
	return func() tea.Msg {
		page, err := c.AlbumsPage(cursor, 200)
		items := make([]item, len(page.Albums))
		for i, a := range page.Albums {
			items[i] = albumItem(a)
		}
		return albumsPageLoadedMsg{screenID: id, items: items, nextCursor: page.NextCursor, append: cursor != "", err: err}
	}
}

func newAlbumTracksScreen(c *Client, album Album) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenAlbumTracks, title: album.Name, ctx: album.ID, list: newList(album.Name)}
	id := s.id
	return s, func() tea.Msg {
		tracks, err := c.AlbumTracks(album.ID)
		items := make([]item, len(tracks))
		for i, t := range tracks {
			items[i] = trackItem(t)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newGenresScreen(c *Client) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenGenres, title: "Genres", list: newCompactList("Genres")}
	id := s.id
	return s, func() tea.Msg {
		genres, err := c.Genres()
		items := make([]item, len(genres))
		for i, g := range genres {
			items[i] = genreItem(g)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newGenreTracksScreen(c *Client, genre string) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenGenreTracks, title: genre, ctx: genre, list: newList(genre)}
	id := s.id
	return s, func() tea.Msg {
		tracks, err := c.GenreTracks(genre)
		items := make([]item, len(tracks))
		for i, t := range tracks {
			items[i] = trackItem(t)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newQueueScreen(c *Client) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenQueue, title: "Queue", list: newList("Queue")}
	return s, loadQueue(c, s.id)
}

func loadQueue(c *Client, id int) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.Queue()
		items := make([]item, len(tracks))
		for i, t := range tracks {
			items[i] = trackItem(t)
		}
		return queueLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newPlaylistsScreen(c *Client) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenPlaylists, title: "Playlists", list: newCompactList("Playlists")}
	id := s.id
	return s, func() tea.Msg {
		names, err := c.Playlists()
		items := make([]item, len(names))
		for i, n := range names {
			items[i] = playlistItem(n)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}

func newPlaylistTracksScreen(c *Client, name string) (screen, tea.Cmd) {
	s := screen{id: newScreenID(), kind: screenPlaylistTracks, title: name, ctx: name, list: newList(name)}
	id := s.id
	return s, func() tea.Msg {
		tracks, err := c.Playlist(name)
		items := make([]item, len(tracks))
		for i, t := range tracks {
			items[i] = trackItem(t)
		}
		return itemsLoadedMsg{screenID: id, items: items, err: err}
	}
}


package main

import (
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// queueDelegate renders each queue row as aligned Artist / Track / Album /
// Duration columns instead of the title+description pair the other screens
// use. Track and Album get fixed base widths; Track absorbs whatever space
// is left over (growing on a wide terminal, shrinking first as it narrows),
// and Album only starts shrinking once Track has hit its floor.
type queueDelegate struct{}

const (
	queueDurWidth = 5 // fits up to "99:59"
	queueGap      = 2 // spaces between columns

	queueArtistBase = 18
	queueAlbumBase  = 20

	queueArtistMin = 8
	queueAlbumMin  = 8
	queueTrackMin  = 6
)

func (queueDelegate) Height() int                         { return 1 }
func (queueDelegate) Spacing() int                        { return 0 }
func (queueDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

// queueColumnWidths splits the available width into artist/track/album
// columns (duration is fixed). Track shrinks first as space runs out, then
// album; artist only gives way once both are already at their floor.
func queueColumnWidths(avail int) (artistW, trackW, albumW int) {
	avail -= queueDurWidth + queueGap*3
	if avail < 0 {
		avail = 0
	}

	artistW, albumW = queueArtistBase, queueAlbumBase
	trackW = avail - artistW - albumW

	if trackW < queueTrackMin {
		deficit := queueTrackMin - trackW
		shrink := min(deficit, max(albumW-queueAlbumMin, 0))
		albumW -= shrink
		trackW += shrink
		deficit -= shrink

		if deficit > 0 {
			shrink = min(deficit, max(artistW-queueArtistMin, 0))
			artistW -= shrink
			trackW += shrink
		}
	}
	if trackW < 0 {
		trackW = 0
	}
	return artistW, trackW, albumW
}

// padCell truncates s to fit width w (adding an ellipsis if it's cut) and
// pads it out to exactly w cells so columns line up down the list.
func padCell(s string, w int, alignRight bool) string {
	if w <= 0 {
		return ""
	}
	tail := "…"
	if alignRight {
		tail = ""
	}
	s = ansi.Truncate(s, w, tail)
	if pad := w - ansi.StringWidth(s); pad > 0 {
		if alignRight {
			s = strings.Repeat(" ", pad) + s
		} else {
			s += strings.Repeat(" ", pad)
		}
	}
	return s
}

func (queueDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok || it.track == nil {
		return
	}
	track := it.track

	styles := list.NewDefaultItemStyles()
	textwidth := m.Width() - styles.NormalTitle.GetPaddingLeft() - styles.NormalTitle.GetPaddingRight()
	if textwidth <= 0 {
		return
	}

	artistW, trackW, albumW := queueColumnWidths(textwidth)

	fav := ""
	if track.Favorite {
		fav = "♥ "
	}

	gap := strings.Repeat(" ", queueGap)
	line := padCell(track.Artist, artistW, false) + gap +
		padCell(fav+track.Title, trackW, false) + gap +
		padCell(track.Album, albumW, false) + gap +
		padCell(formatDuration(track.DurationMS), queueDurWidth, true)

	switch {
	case m.FilterState() == list.Filtering && m.FilterValue() == "":
		line = styles.DimmedTitle.Render(line)
	case index == m.Index() && m.FilterState() != list.Filtering:
		line = styles.SelectedTitle.Render(line)
	default:
		line = styles.NormalTitle.Render(line)
	}

	io.WriteString(w, line)
}

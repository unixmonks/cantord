package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func action(text string, err error, refreshQueue bool) tea.Msg {
	return actionResultMsg{text: text, err: err, refreshQueue: refreshQueue}
}

func cmdTogglePlayPause(c *Client, playing bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if playing {
			err = c.Pause()
		} else {
			err = c.Play()
		}
		return action("", err, false)
	}
}

func cmdNext(c *Client) tea.Cmd {
	return func() tea.Msg { return action("", c.Next(), false) }
}

func cmdPrev(c *Client) tea.Cmd {
	return func() tea.Msg { return action("", c.Prev(), false) }
}

func cmdVolume(c *Client, current float64, delta float64) tea.Cmd {
	return func() tea.Msg {
		v := clampVolume(current + delta)
		return action(fmt.Sprintf("volume %.0f%%", v), c.SetVolume(v), false)
	}
}

func cmdToggleMute(c *Client, muted bool) tea.Cmd {
	return func() tea.Msg { return action("", c.SetMute(!muted), false) }
}

func cmdToggleShuffle(c *Client, on bool) tea.Cmd {
	return func() tea.Msg {
		next := !on
		text := "shuffle off"
		if next {
			text = "shuffle on"
		}
		return action(text, c.SetShuffle(next), false)
	}
}

func nextRepeatMode(mode string) string {
	switch mode {
	case "off", "":
		return "all"
	case "all":
		return "one"
	default:
		return "off"
	}
}

func cmdCycleRepeat(c *Client, mode string) tea.Cmd {
	return func() tea.Msg {
		next := nextRepeatMode(mode)
		return action("repeat "+next, c.CycleRepeat(next), false)
	}
}

func cmdEnqueueTrack(c *Client, id string) tea.Cmd {
	return func() tea.Msg {
		t, err := c.Enqueue(id)
		return action(fmt.Sprintf("queued: %s — %s", t.Artist, t.Title), err, true)
	}
}

func cmdPlayNow(c *Client, id string) tea.Cmd {
	return func() tea.Msg {
		t, err := c.PlayNow(id)
		return action(fmt.Sprintf("playing: %s — %s", t.Artist, t.Title), err, true)
	}
}

// clearAndEnqueue replaces the whole queue with ids, in order, without
// starting playback — the search overlay's "queue my selections" commit
// uses this directly; clearAndQueue below layers autoplay on top for the
// browsing screens' "play this now" actions.
func clearAndEnqueue(c *Client, ids []string) (int, error) {
	if err := c.ClearQueue(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := c.Enqueue(id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// clearAndQueue replaces the whole queue with ids, in order, and starts
// playback at the first one.
func clearAndQueue(c *Client, ids []string) (int, error) {
	n, err := clearAndEnqueue(c, ids)
	if err != nil || n == 0 {
		return n, err
	}
	return n, c.PlayIndex(0)
}

// cmdPlayAlbum is Enter on an album item: replace the queue with the whole
// album, in track order, and start playing it from the top.
func cmdPlayAlbum(c *Client, albumID, label string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.AlbumTracks(albumID)
		if err != nil {
			return action("", err, false)
		}
		ids := make([]string, len(tracks))
		for i, t := range tracks {
			ids[i] = t.ID
		}
		n, err := clearAndQueue(c, ids)
		return action(fmt.Sprintf("playing %s (%d tracks)", label, n), err, true)
	}
}

// cmdPlayTracksFrom is Enter on a track within a browsing screen (album
// tracks, genre tracks, playlist tracks): replace the queue with that track
// and everything listed under it, and start playing from it.
func cmdPlayTracksFrom(c *Client, ids []string, label string) tea.Cmd {
	return func() tea.Msg {
		n, err := clearAndQueue(c, ids)
		return action(fmt.Sprintf("playing %s (%d tracks)", label, n), err, true)
	}
}

func cmdEnqueueAlbum(c *Client, albumID, label string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.AlbumTracks(albumID)
		if err != nil {
			return action("", err, false)
		}
		for _, t := range tracks {
			if _, err := c.Enqueue(t.ID); err != nil {
				return action("", err, true)
			}
		}
		return action(fmt.Sprintf("queued %d tracks from %s", len(tracks), label), nil, true)
	}
}

func cmdEnqueuePlaylist(c *Client, name string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.Playlist(name)
		if err != nil {
			return action("", err, false)
		}
		for _, t := range tracks {
			if _, err := c.Enqueue(t.ID); err != nil {
				return action("", err, true)
			}
		}
		return action(fmt.Sprintf("queued %d tracks from %s", len(tracks), name), nil, true)
	}
}

func cmdRemoveQueueIndex(c *Client, idx int) tea.Cmd {
	return func() tea.Msg { return action("removed", c.RemoveQueueIndex(idx), true) }
}

func cmdMoveQueue(c *Client, from, to int) tea.Cmd {
	return func() tea.Msg { return action("", c.MoveQueue(from, to), true) }
}

func cmdPlayQueueIndex(c *Client, idx int) tea.Cmd {
	return func() tea.Msg { return action("", c.PlayIndex(idx), false) }
}

func cmdAddToPlaylist(c *Client, name, trackID string) tea.Cmd {
	return func() tea.Msg {
		return action(fmt.Sprintf("added to %q", name), c.AddToPlaylist(name, trackID), false)
	}
}

func cmdRemoveFromPlaylist(c *Client, name, trackID string) tea.Cmd {
	return func() tea.Msg { return action("removed from playlist", c.RemoveFromPlaylist(name, trackID), false) }
}

// --- footer rendering ---

func renderFooter(st Status, width int, connected bool) string {
	conn := connectedStyle.Render("●")
	if !connected {
		conn = disconnectedStyle.Render("●")
	}

	// footerStyle pads 1 column on each side, so content must be sized to
	// width-2: sizing it to the full width made lipgloss wrap the 2-column
	// overflow onto an extra line, which pushed the tab bar and list title
	// off the top of the screen.
	inner := width - 2
	if inner < 1 {
		inner = 1
	}

	if st.Track == nil {
		state := st.State
		if state == "" {
			state = "idle"
		}
		line1 := lipglossJoin(fmt.Sprintf("%s %s", footerLabelStyle.Render("♪"), strings.ToUpper(state)), conn, inner)
		return footerStyle.Width(width).Render(line1 + "\n")
	}

	icon := "▶"
	if st.State != "playing" {
		icon = "⏸"
	}
	track := fmt.Sprintf("%s %s — %s", icon, st.Track.Artist, st.Track.Title)

	pos := formatDuration(st.PositionMS)
	dur := formatDuration(st.DurationMS)

	flags := ""
	if st.Shuffle {
		flags += " ⇄"
	}
	if st.Repeat != "" && st.Repeat != "off" {
		flags += " ⟳" + st.Repeat[:1]
	}
	if st.Muted {
		flags += " ✕"
	}

	// Size the bar to whatever room is left after the rest of the line, so
	// the total always fits inner instead of guessing at a fixed budget.
	suffix := fmt.Sprintf("  %s / %s   vol %.0f%%%s", pos, dur, st.Volume, flags)
	barWidth := inner - 2 - lipgloss.Width(suffix)
	if barWidth < 10 {
		barWidth = 10
	}
	bar := progressBar(st.PositionMS, st.DurationMS, barWidth)

	line1 := lipglossJoin(track, conn, inner)
	line2 := bar + suffix

	return footerStyle.Width(width).Render(line1 + "\n" + line2)
}

func progressBar(posMS, durMS, width int) string {
	if durMS <= 0 {
		durMS = 1
	}
	filled := width * posMS / durMS
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// lipglossJoin pads `left` and right-aligns `right` within width, truncating
// left if the terminal is too narrow to fit both.
func lipglossJoin(left, right string, width int) string {
	room := width - lipgloss.Width(right) - 1
	if room < 0 {
		room = 0
	}
	if lipgloss.Width(left) > room {
		left = ansi.Truncate(left, room, "…")
	}
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

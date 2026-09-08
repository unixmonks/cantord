package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
)

func newTable() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

func formatDuration(ms int) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func cmdArtists(c *Client, args []string) error {
	var artists []string
	if err := c.get("/api/artists", &artists); err != nil {
		return err
	}
	for _, a := range artists {
		fmt.Println(a)
	}
	return nil
}

func cmdAlbums(c *Client, args []string) error {
	fs := flag.NewFlagSet("albums", flag.ExitOnError)
	cursor := fs.String("cursor", "", "page cursor from a previous listing's next_cursor")
	letter := fs.String("letter", "", "jump straight to albums starting with this letter (A-Z or #)")
	limit := fs.Int("limit", 50, "page size")
	all := fs.Bool("all", false, "list every page, not just one")
	fs.Parse(args)

	start := *cursor
	if *letter != "" {
		var index map[string]string
		if err := c.get("/api/albums/index", &index); err != nil {
			return err
		}
		start = index[strings.ToUpper(*letter)]
	}

	w := newTable()
	fmt.Fprintln(w, "ALBUM ID\tALBUM\tARTIST\tYEAR\tTRACKS")
	for {
		var page AlbumsPage
		if err := c.get(fmt.Sprintf("/api/albums?cursor=%s&limit=%d", start, *limit), &page); err != nil {
			return err
		}
		for _, a := range page.Albums {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n", a.ID, a.Name, a.AlbumArtist, a.Year, a.TrackCount)
		}
		if !*all || page.NextCursor == "" {
			if page.NextCursor != "" {
				fmt.Fprintf(w, "\t...more; rerun with -cursor %s\t\t\t\n", page.NextCursor)
			}
			break
		}
		start = page.NextCursor
	}
	return w.Flush()
}

func cmdTracks(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl tracks <album_id>")
	}
	var tracks []Track
	if err := c.get("/api/albums/"+args[0]+"/tracks", &tracks); err != nil {
		return err
	}
	w := newTable()
	fmt.Fprintln(w, "TRACK ID\t#\tTITLE\tDURATION\tFORMAT")
	for _, t := range tracks {
		format := t.Codec
		if t.SampleRate > 0 {
			format = fmt.Sprintf("%s %dHz/%dbit", t.Codec, t.SampleRate, t.BitDepth)
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", t.ID, t.TrackNo, t.Title, formatDuration(t.DurationMS), format)
	}
	return w.Flush()
}

func cmdAlbum(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl album <album_id>")
	}
	var a Album
	if err := c.get("/api/albums/"+args[0], &a); err != nil {
		return err
	}
	fmt.Printf("%s — %s\nyear:   %d\ntracks: %d\nart:    %s\n", a.AlbumArtist, a.Name, a.Year, a.TrackCount, a.ArtHash)
	return nil
}

func cmdTrack(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl track <track_id>")
	}
	var t Track
	if err := c.get("/api/tracks/"+args[0], &t); err != nil {
		return err
	}
	format := t.Codec
	if t.SampleRate > 0 {
		format = fmt.Sprintf("%s %dHz/%dbit", t.Codec, t.SampleRate, t.BitDepth)
	}
	fmt.Printf("%s — %s\nalbum:    %s\nduration: %s\nformat:   %s\npath:     %s\n",
		t.Artist, t.Title, t.Album, formatDuration(t.DurationMS), format, t.Path)
	return nil
}

func cmdFind(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl find <query>")
	}
	query := strings.Join(args, " ")

	var result SearchResult
	if err := c.get("/api/search?q="+url.QueryEscape(query), &result); err != nil {
		return err
	}

	if len(result.Albums) > 0 {
		w := newTable()
		fmt.Fprintln(w, "ALBUM ID\tALBUM\tARTIST\tYEAR")
		for _, a := range result.Albums {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", a.ID, a.Name, a.AlbumArtist, a.Year)
		}
		w.Flush()
	}
	if len(result.Tracks) > 0 {
		if len(result.Albums) > 0 {
			fmt.Println()
		}
		w := newTable()
		fmt.Fprintln(w, "TRACK ID\tARTIST\tTITLE\tALBUM")
		for _, t := range result.Tracks {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Artist, t.Title, t.Album)
		}
		w.Flush()
	}
	if len(result.Albums) == 0 && len(result.Tracks) == 0 {
		fmt.Println("no matches")
	}
	return nil
}

func cmdStatus(c *Client, args []string) error {
	var st Status
	if err := c.get("/api/status", &st); err != nil {
		return err
	}
	if st.Track == nil {
		fmt.Printf("[%s]\n", st.State)
		return nil
	}
	mute := ""
	if st.Muted {
		mute = " (muted)"
	}
	flags := ""
	if st.Shuffle {
		flags += " shuffle"
	}
	if st.Repeat != "" && st.Repeat != "off" {
		flags += " repeat-" + st.Repeat
	}
	fmt.Printf("[%s] %s — %s  (%s / %s)  vol %.0f%%%s%s\n",
		st.State, st.Track.Artist, st.Track.Title,
		formatDuration(st.PositionMS), formatDuration(st.DurationMS), st.Volume, mute, flags)
	return nil
}

func cmdQueue(c *Client, args []string) error {
	var tracks []Track
	if err := c.get("/api/queue", &tracks); err != nil {
		return err
	}
	w := newTable()
	fmt.Fprintln(w, "#\tTRACK ID\tARTIST\tTITLE\tDURATION")
	for i, t := range tracks {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i, t.ID, t.Artist, t.Title, formatDuration(t.DurationMS))
	}
	return w.Flush()
}

// play enqueues a track (auto-plays if the queue was empty) when given an
// ID, or just resumes playback when given none.
func cmdPlay(c *Client, args []string) error {
	if len(args) == 0 {
		return c.post("/api/playback/play", nil, nil)
	}
	var t Track
	if err := c.post("/api/queue", map[string]string{"track_id": args[0]}, &t); err != nil {
		return err
	}
	fmt.Printf("queued: %s — %s\n", t.Artist, t.Title)
	return nil
}

func cmdPlayAlbum(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl play-album <album_id>")
	}
	var tracks []Track
	if err := c.get("/api/albums/"+args[0]+"/tracks", &tracks); err != nil {
		return err
	}
	for _, t := range tracks {
		if err := c.post("/api/queue", map[string]string{"track_id": t.ID}, nil); err != nil {
			return fmt.Errorf("enqueuing %s: %w", t.Title, err)
		}
	}
	fmt.Printf("queued %d tracks\n", len(tracks))
	return nil
}

func cmdPlayNext(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl playnext <track_id>")
	}
	var t Track
	if err := c.post("/api/queue/next", map[string]string{"track_id": args[0]}, &t); err != nil {
		return err
	}
	fmt.Printf("playing next: %s — %s\n", t.Artist, t.Title)
	return nil
}

func cmdMove(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl move <from_index> <to_index>")
	}
	from, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	to, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	return c.post("/api/queue/move", map[string]int{"from": from, "to": to}, nil)
}

func cmdMute(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl mute <on|off>")
	}
	return c.post("/api/playback/mute", map[string]bool{"muted": args[0] == "on"}, nil)
}

func cmdShuffle(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl shuffle <on|off>")
	}
	return c.post("/api/playback/shuffle", map[string]bool{"shuffle": args[0] == "on"}, nil)
}

func cmdRepeat(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl repeat <off|one|all>")
	}
	return c.post("/api/playback/repeat", map[string]string{"mode": args[0]}, nil)
}

func printTrackTable(tracks []Track) error {
	w := newTable()
	fmt.Fprintln(w, "TRACK ID\tARTIST\tTITLE\tALBUM\tFAV\tRATING")
	for _, t := range tracks {
		fav := ""
		if t.Favorite {
			fav = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", t.ID, t.Artist, t.Title, t.Album, fav, t.Rating)
	}
	return w.Flush()
}

func cmdRecent(c *Client, args []string) error {
	fs := flag.NewFlagSet("recent", flag.ExitOnError)
	limit := fs.Int("limit", 50, "how many tracks to show")
	fs.Parse(args)

	var tracks []Track
	if err := c.get(fmt.Sprintf("/api/tracks/recent?limit=%d", *limit), &tracks); err != nil {
		return err
	}
	return printTrackTable(tracks)
}

func cmdRecentlyPlayed(c *Client, args []string) error {
	fs := flag.NewFlagSet("recently-played", flag.ExitOnError)
	limit := fs.Int("limit", 50, "how many tracks to show")
	fs.Parse(args)

	var tracks []Track
	if err := c.get(fmt.Sprintf("/api/tracks/recently-played?limit=%d", *limit), &tracks); err != nil {
		return err
	}
	return printTrackTable(tracks)
}

func cmdGenres(c *Client, args []string) error {
	var genres []string
	if err := c.get("/api/genres", &genres); err != nil {
		return err
	}
	for _, g := range genres {
		fmt.Println(g)
	}
	return nil
}

func cmdGenreTracks(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl genre <name>")
	}
	var tracks []Track
	if err := c.get("/api/genres/"+url.QueryEscape(args[0])+"/tracks", &tracks); err != nil {
		return err
	}
	return printTrackTable(tracks)
}

func cmdStats(c *Client, args []string) error {
	var st Stats
	if err := c.get("/api/library/stats", &st); err != nil {
		return err
	}
	fmt.Printf("tracks:   %d\nalbums:   %d\nartists:  %d\nsize:     %.2f GB\nduration: %s\n",
		st.Tracks, st.Albums, st.Artists, float64(st.TotalSize)/(1<<30), formatDuration(int(st.TotalDurationMS)))
	return nil
}

func cmdFavorites(c *Client, args []string) error {
	var tracks []Track
	if err := c.get("/api/tracks/favorites", &tracks); err != nil {
		return err
	}
	return printTrackTable(tracks)
}

func cmdFav(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl fav <track_id> [on|off]")
	}
	fav := true
	if len(args) >= 2 {
		fav = args[1] == "on"
	}
	return c.post("/api/tracks/"+args[0]+"/favorite", map[string]bool{"favorite": fav}, nil)
}

func cmdRate(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl rate <track_id> <0-5>")
	}
	rating, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	return c.post("/api/tracks/"+args[0]+"/rating", map[string]int{"rating": rating}, nil)
}

func cmdPlaylistAdd(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl playlist-add <name> <track_id>")
	}
	return c.post("/api/playlists/"+args[0]+"/tracks", map[string]string{"track_id": args[1]}, nil)
}

func cmdPlaylistRemove(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl playlist-remove <name> <track_id>")
	}
	return c.delete("/api/playlists/"+args[0]+"/tracks/"+args[1], nil)
}

func cmdPlaylistRename(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl playlist-rename <old_name> <new_name>")
	}
	return c.post("/api/playlists/"+args[0]+"/rename", map[string]string{"new_name": args[1]}, nil)
}

func cmdVersion(c *Client, args []string) error {
	var v struct {
		Version string `json:"version"`
	}
	if err := c.get("/api/version", &v); err != nil {
		return err
	}
	fmt.Println(v.Version)
	return nil
}

func cmdHealth(c *Client, args []string) error {
	var h struct {
		Status string `json:"status"`
	}
	if err := c.get("/api/health", &h); err != nil {
		return err
	}
	fmt.Println(h.Status)
	return nil
}

func cmdSimple(path string) func(*Client, []string) error {
	return func(c *Client, args []string) error { return c.post(path, nil, nil) }
}

func cmdSeek(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl seek <seconds>")
	}
	seconds, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return err
	}
	return c.post("/api/playback/seek", map[string]float64{"position_seconds": seconds}, nil)
}

func cmdVolume(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl volume <0-100>")
	}
	vol, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return err
	}
	return c.post("/api/playback/volume", map[string]float64{"volume": vol}, nil)
}

func cmdClear(c *Client, args []string) error {
	return c.post("/api/queue/clear", nil, nil)
}

func cmdRemove(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl remove <queue_index>")
	}
	return c.delete("/api/queue/"+args[0], nil)
}

func cmdPlayIndex(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl playidx <queue_index>")
	}
	return c.post("/api/queue/"+args[0]+"/play", nil, nil)
}

func cmdPlaylists(c *Client, args []string) error {
	var names []string
	if err := c.get("/api/playlists", &names); err != nil {
		return err
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func cmdPlaylistSave(c *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantordctl playlist-save <name> <track_id> [track_id...]")
	}
	return c.post("/api/playlists", map[string]any{"name": args[0], "track_ids": args[1:]}, nil)
}

func cmdPlaylistGet(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl playlist-get <name>")
	}
	var out struct {
		Name     string   `json:"name"`
		TrackIDs []string `json:"track_ids"`
	}
	if err := c.get("/api/playlists/"+args[0], &out); err != nil {
		return err
	}
	for _, id := range out.TrackIDs {
		fmt.Println(id)
	}
	return nil
}

func cmdPlaylistDelete(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl playlist-delete <name>")
	}
	return c.delete("/api/playlists/"+args[0], nil)
}

func cmdScan(c *Client, args []string) error {
	return c.post("/api/library/scan", nil, nil)
}

func cmdScanStatus(c *Client, args []string) error {
	var p ScanProgress
	if err := c.get("/api/library/scan/status", &p); err != nil {
		return err
	}
	fmt.Printf("running=%v processed=%d/%d added=%d skipped=%d failed=%d\n",
		p.Running, p.Processed, p.Total, p.AddedOrUpdated, p.Skipped, p.Failed)
	return nil
}

// events streams the daemon's SSE feed to stdout as "type: data" lines —
// handy for watching status/queue/scan updates live while exercising the
// rest of the CLI from another terminal.
func cmdEvents(c *Client, args []string) error {
	resp, err := http.Get(c.base + "/api/events")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var event string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			fmt.Printf("%s: %s\n", event, strings.TrimPrefix(line, "data: "))
		}
	}
	return scanner.Err()
}

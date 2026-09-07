package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
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

// find is a client-side convenience: cantord has no /search endpoint, so
// this pages through every album and filters locally. Fine at personal-
// library scale; not meant for huge collections.
func cmdFind(c *Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantordctl find <query>")
	}
	query := strings.ToLower(strings.Join(args, " "))

	w := newTable()
	fmt.Fprintln(w, "ALBUM ID\tALBUM\tARTIST\tYEAR")
	cursor := ""
	for {
		var page AlbumsPage
		if err := c.get(fmt.Sprintf("/api/albums?cursor=%s&limit=200", cursor), &page); err != nil {
			return err
		}
		for _, a := range page.Albums {
			haystack := strings.ToLower(a.Name + " " + a.AlbumArtist)
			if strings.Contains(haystack, query) {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", a.ID, a.Name, a.AlbumArtist, a.Year)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return w.Flush()
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
	fmt.Printf("[%s] %s — %s  (%s / %s)  vol %.0f%%\n",
		st.State, st.Track.Artist, st.Track.Title,
		formatDuration(st.PositionMS), formatDuration(st.DurationMS), st.Volume)
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

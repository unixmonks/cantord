package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin wrapper over cantord's HTTP/JSON API — deliberately
// independent of the daemon's internal packages, the same as cantordctl,
// since the TUI only ever talks over the wire like any other frontend.
type Client struct {
	base string
	http *http.Client
	// stream has no overall Timeout: it's used for the long-lived SSE GET,
	// which http.Client.Timeout would otherwise cut off mid-connection
	// (that timeout applies to the whole request, including body reads).
	stream *http.Client
}

func NewClient(base string) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		http:   &http.Client{Timeout: 15 * time.Second},
		stream: &http.Client{},
	}
}

func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", c.base, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("%s: %s", path, apiErr.Error)
		}
		return fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) get(path string, out any) error { return c.do(http.MethodGet, path, nil, out) }
func (c *Client) post(path string, body, out any) error {
	return c.do(http.MethodPost, path, body, out)
}
func (c *Client) delete(path string, out any) error { return c.do(http.MethodDelete, path, nil, out) }

// --- API response shapes, mirroring the server's JSON exactly ---

type Album struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	ArtHash     string `json:"art_hash,omitempty"`
	TrackCount  int    `json:"track_count"`
}

type AlbumsPage struct {
	Albums     []Album `json:"albums"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Track struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist"`
	AlbumID     string `json:"album_id"`
	ArtHash     string `json:"art_hash,omitempty"`
	TrackNo     int    `json:"track_no"`
	DiscNo      int    `json:"disc_no"`
	Year        int    `json:"year"`
	Genre       string `json:"genre"`
	DurationMS  int    `json:"duration_ms"`
	Codec       string `json:"codec"`
	SampleRate  int    `json:"sample_rate"`
	BitDepth    int    `json:"bit_depth"`
	Channels    int    `json:"channels"`
	AddedAt     int64  `json:"added_at"`
	Favorite    bool   `json:"favorite"`
	Rating      int    `json:"rating"`
}

type Status struct {
	State      string  `json:"state"`
	Track      *Track  `json:"track,omitempty"`
	QueueIndex int     `json:"queue_index"`
	PositionMS int     `json:"position_ms"`
	DurationMS int     `json:"duration_ms"`
	Volume     float64 `json:"volume"`
	Muted      bool    `json:"muted"`
	Shuffle    bool    `json:"shuffle"`
	Repeat     string  `json:"repeat"`
}

type SearchResult struct {
	Artists []string `json:"artists"`
	Albums  []Album  `json:"albums"`
	Tracks  []Track  `json:"tracks"`
}

// --- calls used by the TUI ---

func (c *Client) Artists() ([]string, error) {
	var artists []string
	err := c.get("/api/artists", &artists)
	return artists, err
}

func (c *Client) Genres() ([]string, error) {
	var genres []string
	err := c.get("/api/genres", &genres)
	return genres, err
}

func (c *Client) GenreTracks(genre string) ([]Track, error) {
	var tracks []Track
	err := c.get("/api/genres/"+url.PathEscape(genre)+"/tracks", &tracks)
	return tracks, err
}

func (c *Client) AlbumsPage(cursor string, limit int) (AlbumsPage, error) {
	var page AlbumsPage
	err := c.get(fmt.Sprintf("/api/albums?cursor=%s&limit=%d", url.QueryEscape(cursor), limit), &page)
	return page, err
}

func (c *Client) AlbumIndex() (map[string]string, error) {
	var idx map[string]string
	err := c.get("/api/albums/index", &idx)
	return idx, err
}

func (c *Client) AlbumTracks(albumID string) ([]Track, error) {
	var tracks []Track
	err := c.get("/api/albums/"+url.PathEscape(albumID)+"/tracks", &tracks)
	return tracks, err
}

// AlbumsByArtist has no dedicated endpoint, so it reuses search (which
// substring-matches album_artist) and keeps only exact matches.
func (c *Client) AlbumsByArtist(artist string) ([]Album, error) {
	var result SearchResult
	if err := c.get("/api/search?q="+url.QueryEscape(artist)+"&limit=500", &result); err != nil {
		return nil, err
	}
	albums := result.Albums[:0]
	for _, a := range result.Albums {
		if a.AlbumArtist == artist {
			albums = append(albums, a)
		}
	}
	return albums, nil
}

func (c *Client) Search(query string) (SearchResult, error) {
	var result SearchResult
	err := c.get("/api/search?q="+url.QueryEscape(query)+"&limit=50", &result)
	return result, err
}

// Art fetches album art bytes for a content-addressed hash (Track.ArtHash /
// Album.ArtHash). "thumb" is plenty for a terminal-cell rendering.
func (c *Client) Art(hash string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+"/art/"+url.PathEscape(hash)+"?size=thumb", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", c.base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("art %s: status %d", hash, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) Status() (Status, error) {
	var st Status
	err := c.get("/api/status", &st)
	return st, err
}

func (c *Client) Queue() ([]Track, error) {
	var tracks []Track
	err := c.get("/api/queue", &tracks)
	return tracks, err
}

func (c *Client) Enqueue(trackID string) (Track, error) {
	var t Track
	err := c.post("/api/queue", map[string]string{"track_id": trackID}, &t)
	return t, err
}

// PlayNow enqueues a track and immediately jumps playback to it, regardless
// of what else is queued — the "play this now" action, distinct from the
// plain add-to-queue append.
func (c *Client) PlayNow(trackID string) (Track, error) {
	t, err := c.Enqueue(trackID)
	if err != nil {
		return t, err
	}
	q, err := c.Queue()
	if err != nil {
		return t, err
	}
	if len(q) > 0 {
		return t, c.PlayIndex(len(q) - 1)
	}
	return t, nil
}

func (c *Client) PlayIndex(i int) error {
	return c.post(fmt.Sprintf("/api/queue/%d/play", i), nil, nil)
}

func (c *Client) ClearQueue() error {
	return c.post("/api/queue/clear", nil, nil)
}

func (c *Client) RemoveQueueIndex(i int) error {
	return c.delete(fmt.Sprintf("/api/queue/%d", i), nil)
}

func (c *Client) MoveQueue(from, to int) error {
	return c.post("/api/queue/move", map[string]int{"from": from, "to": to}, nil)
}

func (c *Client) Play() error  { return c.post("/api/playback/play", nil, nil) }
func (c *Client) Pause() error { return c.post("/api/playback/pause", nil, nil) }
func (c *Client) Next() error  { return c.post("/api/playback/next", nil, nil) }
func (c *Client) Prev() error  { return c.post("/api/playback/previous", nil, nil) }

func (c *Client) SetVolume(v float64) error {
	return c.post("/api/playback/volume", map[string]float64{"volume": v}, nil)
}

func (c *Client) SetMute(m bool) error {
	return c.post("/api/playback/mute", map[string]bool{"muted": m}, nil)
}

func (c *Client) SetShuffle(on bool) error {
	return c.post("/api/playback/shuffle", map[string]bool{"shuffle": on}, nil)
}

func (c *Client) CycleRepeat(mode string) error {
	return c.post("/api/playback/repeat", map[string]string{"mode": mode}, nil)
}

func (c *Client) Playlists() ([]string, error) {
	var names []string
	err := c.get("/api/playlists", &names)
	return names, err
}

func (c *Client) Playlist(name string) ([]Track, error) {
	var out struct {
		TrackIDs []string `json:"track_ids"`
	}
	if err := c.get("/api/playlists/"+url.PathEscape(name), &out); err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(out.TrackIDs))
	for _, id := range out.TrackIDs {
		var t Track
		if err := c.get("/api/tracks/"+url.PathEscape(id), &t); err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (c *Client) AddToPlaylist(name, trackID string) error {
	return c.post("/api/playlists/"+url.PathEscape(name)+"/tracks", map[string]string{"track_id": trackID}, nil)
}

func (c *Client) RemoveFromPlaylist(name, trackID string) error {
	return c.delete("/api/playlists/"+url.PathEscape(name)+"/tracks/"+url.PathEscape(trackID), nil)
}

func (c *Client) DeletePlaylist(name string) error {
	return c.delete("/api/playlists/"+url.PathEscape(name), nil)
}

// StreamEvents connects to the daemon's SSE feed and pushes decoded events
// onto ch until the context is cancelled or the connection drops. The
// caller is expected to reconnect (main loop does, with a short backoff).
func (c *Client) StreamEvents(ctx context.Context, ch chan<- sseEvent) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/events", nil)
	if err != nil {
		return err
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var evt sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			evt.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			evt.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if evt.name != "" {
				select {
				case ch <- evt:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			evt = sseEvent{}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

type sseEvent struct {
	name string
	data string
}

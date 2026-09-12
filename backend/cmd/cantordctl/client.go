package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a thin wrapper over cantord's HTTP/JSON API — deliberately
// independent of the daemon's internal packages, since it only ever talks
// over the wire, the same as any other frontend would.
type Client struct {
	base string
	http *http.Client
}

func NewClient(base string) *Client {
	return &Client{base: base, http: &http.Client{Timeout: 15 * time.Second}}
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
			return fmt.Errorf("%s %s: %s (status %d)", method, path, apiErr.Error, resp.StatusCode)
		}
		return fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
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
	Available   bool   `json:"available"`
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

type Stats struct {
	Tracks          int   `json:"tracks"`
	Unavailable     int   `json:"unavailable"`
	Albums          int   `json:"albums"`
	Artists         int   `json:"artists"`
	TotalSize       int64 `json:"total_size_bytes"`
	TotalDurationMS int64 `json:"total_duration_ms"`
}

type SearchResult struct {
	Albums []Album `json:"albums"`
	Tracks []Track `json:"tracks"`
}

type ScanProgress struct {
	Running         bool   `json:"running"`
	Total           int    `json:"total"`
	Processed       int    `json:"processed"`
	AddedOrUpdated  int    `json:"added_or_updated"`
	Skipped         int    `json:"skipped_unchanged"`
	MarkedMissing   int    `json:"marked_missing"`
	MarkedAvailable int    `json:"marked_available"`
	Unavailable     int    `json:"unavailable"`
	Failed          int    `json:"failed"`
	CurrentPath     string `json:"current_path,omitempty"`
}

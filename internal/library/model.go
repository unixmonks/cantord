// Package library indexes an on-disk music collection into a queryable,
// content-addressed cache (metadata in SQLite, art blobs on disk).
package library

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
	Size        int64  `json:"size"`
	MTime       int64  `json:"mtime"`
}

type Album struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	SortKey     string `json:"-"`
	ArtHash     string `json:"art_hash,omitempty"`
	TrackCount  int    `json:"track_count"`
}

type Page struct {
	Albums     []Album `json:"albums"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

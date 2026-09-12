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
	Size        int64  `json:"size"`
	MTime       int64  `json:"mtime"`
	AddedAt     int64  `json:"added_at"`
	Favorite    bool   `json:"favorite"`
	Rating      int    `json:"rating"`

	// Available is false once a scan walked this track's music directory
	// cleanly and didn't find the file there anymore (e.g. an NFS mount was
	// down and has since come back empty, or the file was actually
	// deleted). The row is kept either way — scans never delete tracks on
	// their own, see Store.PruneUnavailable — so playback and browsing keep
	// working from cached metadata even while the backing storage is gone.
	Available    bool  `json:"available"`
	MissingSince int64 `json:"missing_since,omitempty"`
}

// Stats is a whole-library summary — cheap aggregate counts for a client's
// "about my library" view.
type Stats struct {
	Tracks          int   `json:"tracks"`
	Unavailable     int   `json:"unavailable"`
	Albums          int   `json:"albums"`
	Artists         int   `json:"artists"`
	TotalSize       int64 `json:"total_size_bytes"`
	TotalDurationMS int64 `json:"total_duration_ms"`
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

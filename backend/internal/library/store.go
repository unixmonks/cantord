package library

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS albums (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	album_artist TEXT NOT NULL,
	year INTEGER,
	sort_key TEXT NOT NULL,
	art_hash TEXT,
	track_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_albums_sort ON albums(sort_key, id);

CREATE TABLE IF NOT EXISTS tracks (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	title TEXT,
	artist TEXT,
	album TEXT,
	album_artist TEXT,
	album_id TEXT NOT NULL,
	track_no INTEGER,
	disc_no INTEGER,
	year INTEGER,
	genre TEXT,
	duration_ms INTEGER,
	codec TEXT,
	sample_rate INTEGER,
	bit_depth INTEGER,
	channels INTEGER,
	size INTEGER,
	mtime INTEGER
);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id, disc_no, track_no);

CREATE TABLE IF NOT EXISTS playlists (
	name TEXT PRIMARY KEY,
	track_ids TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS play_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	track_id TEXT NOT NULL,
	played_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_play_history_track ON play_history(track_id, played_at);

CREATE TABLE IF NOT EXISTS favorites (
	track_id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ratings (
	track_id TEXT PRIMARY KEY,
	rating INTEGER NOT NULL
);

-- Singleton row (id always 1): the play queue and playback position, so a
-- restart or crash resumes where the user left off instead of starting
-- empty.
CREATE TABLE IF NOT EXISTS queue_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	track_ids TEXT NOT NULL,
	position INTEGER NOT NULL,
	position_ms INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_conversations (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	conversation_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_messages_conv ON ai_messages(conversation_id, id);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // modernc.org/sqlite: keep writes serialized, avoid SQLITE_BUSY
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	if err := addColumnIfMissing(db, "tracks", "added_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return &Store{db: db}, nil
}

// addColumnIfMissing is a minimal ad-hoc migration helper: CREATE TABLE IF
// NOT EXISTS above only shapes brand-new databases, so a column added after
// the fact needs an explicit, idempotent ALTER TABLE.
func addColumnIfMissing(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, decl))
	return err
}

// trackColumns/trackJoins let every query that returns Track rows share one
// column list and one join shape (favorites/ratings are sparse side tables,
// not columns on tracks itself) instead of drifting out of sync.
const trackColumns = `t.id, t.path, t.title, t.artist, t.album, t.album_artist, t.album_id, a.art_hash, t.track_no, t.disc_no,
	t.year, t.genre, t.duration_ms, t.codec, t.sample_rate, t.bit_depth, t.channels, t.size, t.mtime, t.added_at,
	CASE WHEN f.track_id IS NULL THEN 0 ELSE 1 END, COALESCE(r.rating, 0)`

const trackJoins = `LEFT JOIN favorites f ON f.track_id = t.id LEFT JOIN ratings r ON r.track_id = t.id LEFT JOIN albums a ON a.id = t.album_id`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrack(rs rowScanner) (Track, error) {
	var t Track
	var fav int
	var artHash sql.NullString
	err := rs.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.AlbumID, &artHash,
		&t.TrackNo, &t.DiscNo, &t.Year, &t.Genre, &t.DurationMS, &t.Codec, &t.SampleRate, &t.BitDepth,
		&t.Channels, &t.Size, &t.MTime, &t.AddedAt, &fav, &t.Rating)
	t.Favorite = fav != 0
	t.ArtHash = artHash.String
	return t, err
}

func (s *Store) Close() error { return s.db.Close() }

// TrackFingerprint reports the (size, mtime) cantord last saw for a path, so
// the scanner can skip re-reading tags/probing unchanged files.
func (s *Store) TrackFingerprint(path string) (size, mtime int64, found bool) {
	err := s.db.QueryRow(`SELECT size, mtime FROM tracks WHERE path = ?`, path).Scan(&size, &mtime)
	if err != nil {
		return 0, 0, false
	}
	return size, mtime, true
}

// UpsertTrack writes a track and rolls its album's aggregate fields
// (name/art/track_count) forward. Returns the album ID.
func (s *Store) UpsertTrack(t Track) (string, error) {
	albumArtist := t.AlbumArtist
	if albumArtist == "" {
		albumArtist = t.Artist
	}
	albumID := hashHex(albumArtist, t.Album)
	t.AlbumID = albumID
	if t.ID == "" {
		t.ID = hashHex(t.Path)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO tracks (id, path, title, artist, album, album_artist, album_id, track_no, disc_no, year, genre, duration_ms, codec, sample_rate, bit_depth, channels, size, mtime, added_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
			id=excluded.id, title=excluded.title, artist=excluded.artist, album=excluded.album,
			album_artist=excluded.album_artist, album_id=excluded.album_id, track_no=excluded.track_no,
			disc_no=excluded.disc_no, year=excluded.year, genre=excluded.genre, duration_ms=excluded.duration_ms,
			codec=excluded.codec, sample_rate=excluded.sample_rate, bit_depth=excluded.bit_depth,
			channels=excluded.channels, size=excluded.size, mtime=excluded.mtime
	`, t.ID, t.Path, t.Title, t.Artist, t.Album, albumArtist, albumID, t.TrackNo, t.DiscNo, t.Year, t.Genre,
		t.DurationMS, t.Codec, t.SampleRate, t.BitDepth, t.Channels, t.Size, t.MTime, time.Now().Unix())
	if err != nil {
		return "", fmt.Errorf("upserting track: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO albums (id, name, album_artist, year, sort_key, track_count)
		VALUES (?,?,?,?,?,1)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, album_artist=excluded.album_artist, year=excluded.year, sort_key=excluded.sort_key,
			track_count=(SELECT COUNT(*) FROM tracks WHERE album_id = excluded.id)
	`, albumID, t.Album, albumArtist, t.Year, sortKey(albumDisplayKey(albumArtist, t.Album)))
	if err != nil {
		return "", fmt.Errorf("upserting album: %w", err)
	}

	return albumID, tx.Commit()
}

func albumDisplayKey(albumArtist, album string) string {
	if albumArtist != "" {
		return albumArtist + " " + album
	}
	return album
}

// SetAlbumArt records the content-addressed art hash for an album,
// overwriting whatever was there (used by the enrichment pipeline, which
// only targets albums it already confirmed have no art).
func (s *Store) SetAlbumArt(albumID, artHash string) error {
	_, err := s.db.Exec(`UPDATE albums SET art_hash = ? WHERE id = ?`, artHash, albumID)
	return err
}

// SetAlbumArtIfEmpty sets art only if the album doesn't already have some —
// used by the scanner so embedded art never clobbers a better result an
// external provider already found.
func (s *Store) SetAlbumArtIfEmpty(albumID, artHash string) error {
	_, err := s.db.Exec(`UPDATE albums SET art_hash = ? WHERE id = ? AND (art_hash IS NULL OR art_hash = '')`, artHash, albumID)
	return err
}

// RemoveMissing deletes track rows whose paths are no longer on disk and
// prunes albums left with zero tracks.
func (s *Store) RemoveMissing(seenPaths map[string]bool) (removed int, err error) {
	rows, err := s.db.Query(`SELECT id, path FROM tracks`)
	if err != nil {
		return 0, err
	}
	var stale []string
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			rows.Close()
			return 0, err
		}
		if !seenPaths[path] {
			stale = append(stale, id)
		}
	}
	rows.Close()

	for _, id := range stale {
		if _, err := s.db.Exec(`DELETE FROM tracks WHERE id = ?`, id); err != nil {
			return removed, err
		}
		removed++
	}
	if _, err := s.db.Exec(`
		UPDATE albums SET track_count = (SELECT COUNT(*) FROM tracks WHERE album_id = albums.id)
	`); err != nil {
		return removed, err
	}
	_, err = s.db.Exec(`DELETE FROM albums WHERE track_count = 0`)
	return removed, err
}

type albumCursor struct {
	SortKey string `json:"k"`
	ID      string `json:"i"`
}

func encodeCursor(c albumCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (albumCursor, bool) {
	if s == "" {
		return albumCursor{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return albumCursor{}, false
	}
	var c albumCursor
	if json.Unmarshal(b, &c) != nil {
		return albumCursor{}, false
	}
	return c, true
}

// ListAlbums returns one page of albums in alphabetical order. cursor is
// inclusive: pass "" for the first page, or a cursor from a previous page's
// NextCursor / from AlphabetIndex to jump straight to a letter.
func (s *Store) ListAlbums(cursor string, limit int) (Page, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if c, ok := decodeCursor(cursor); ok {
		rows, err = s.db.Query(`
			SELECT id, name, album_artist, year, sort_key, art_hash, track_count FROM albums
			WHERE (sort_key, id) >= (?, ?)
			ORDER BY sort_key, id LIMIT ?
		`, c.SortKey, c.ID, limit+1)
	} else {
		rows, err = s.db.Query(`
			SELECT id, name, album_artist, year, sort_key, art_hash, track_count FROM albums
			ORDER BY sort_key, id LIMIT ?
		`, limit+1)
	}
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var a Album
		var artHash sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumArtist, &a.Year, &a.SortKey, &artHash, &a.TrackCount); err != nil {
			return Page{}, err
		}
		a.ArtHash = artHash.String
		albums = append(albums, a)
	}

	page := Page{Albums: albums}
	if len(albums) > limit {
		last := albums[limit]
		page.Albums = albums[:limit]
		page.NextCursor = encodeCursor(albumCursor{SortKey: last.SortKey, ID: last.ID})
	}
	return page, nil
}

// AlphabetIndex returns, for each leading letter/"#" bucket present in the
// library, a cursor that jumps ListAlbums straight to that bucket.
func (s *Store) AlphabetIndex() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT id, sort_key FROM albums ORDER BY sort_key, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	index := map[string]string{}
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		letter := letterBucket(key)
		if _, seen := index[letter]; !seen {
			index[letter] = encodeCursor(albumCursor{SortKey: key, ID: id})
		}
	}
	return index, nil
}

// SearchResult is the combined hit set for a query: matching artists,
// albums, and tracks, so a client can jump straight to any of them.
type SearchResult struct {
	Artists []string `json:"artists"`
	Albums  []Album  `json:"albums"`
	Tracks  []Track  `json:"tracks"`
}

// Search does a case-insensitive substring match (SQLite's LIKE is
// case-insensitive for ASCII by default) across album name/artist and
// track title/artist/album. It's intentionally simple — no ranking, no
// FTS index — which is plenty for a personal-library-scale collection;
// revisit with FTS5 if that stops being true.
func (s *Store) Search(query string, limit int) (SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	pattern := "%" + query + "%"

	artistRows, err := s.db.Query(`
		SELECT DISTINCT album_artist FROM albums
		WHERE album_artist LIKE ? AND album_artist != ''
	`, pattern)
	if err != nil {
		return SearchResult{}, err
	}
	defer artistRows.Close()

	var result SearchResult
	for artistRows.Next() {
		var name string
		if err := artistRows.Scan(&name); err != nil {
			return SearchResult{}, err
		}
		result.Artists = append(result.Artists, name)
	}
	sort.Slice(result.Artists, func(i, j int) bool { return sortKey(result.Artists[i]) < sortKey(result.Artists[j]) })
	if len(result.Artists) > limit {
		result.Artists = result.Artists[:limit]
	}

	albumRows, err := s.db.Query(`
		SELECT id, name, album_artist, year, sort_key, art_hash, track_count FROM albums
		WHERE name LIKE ? OR album_artist LIKE ?
		ORDER BY sort_key, id LIMIT ?
	`, pattern, pattern, limit)
	if err != nil {
		return SearchResult{}, err
	}
	defer albumRows.Close()

	for albumRows.Next() {
		var a Album
		var artHash sql.NullString
		if err := albumRows.Scan(&a.ID, &a.Name, &a.AlbumArtist, &a.Year, &a.SortKey, &artHash, &a.TrackCount); err != nil {
			return SearchResult{}, err
		}
		a.ArtHash = artHash.String
		result.Albums = append(result.Albums, a)
	}

	trackRows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.title LIKE ? OR t.artist LIKE ? OR t.album LIKE ? OR t.album_artist LIKE ?
		ORDER BY t.artist, t.album, t.disc_no, t.track_no LIMIT ?
	`, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return SearchResult{}, err
	}
	defer trackRows.Close()

	for trackRows.Next() {
		t, err := scanTrack(trackRows)
		if err != nil {
			return SearchResult{}, err
		}
		result.Tracks = append(result.Tracks, t)
	}

	return result, nil
}

// ListArtists returns the distinct album artists in the library,
// alphabetized the same way albums are (leading "the/a/an" ignored).
// Compilation/various-artist track-level artists aren't split out here —
// this reflects album_artist, matching how the album list groups things.
func (s *Store) ListArtists() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT album_artist FROM albums WHERE album_artist != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	sort.Slice(artists, func(i, j int) bool { return sortKey(artists[i]) < sortKey(artists[j]) })
	return artists, nil
}

func (s *Store) GetAlbum(id string) (Album, bool, error) {
	var a Album
	var artHash sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, album_artist, year, sort_key, art_hash, track_count FROM albums WHERE id = ?
	`, id).Scan(&a.ID, &a.Name, &a.AlbumArtist, &a.Year, &a.SortKey, &artHash, &a.TrackCount)
	if err == sql.ErrNoRows {
		return Album{}, false, nil
	}
	if err != nil {
		return Album{}, false, err
	}
	a.ArtHash = artHash.String
	return a, true, nil
}

func (s *Store) ListTracksForAlbum(albumID string) ([]Track, error) {
	rows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.album_id = ? ORDER BY t.disc_no, t.track_no, t.title
	`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// ListRecentlyAdded returns the most recently scanned-in tracks, newest
// first — driven by added_at (set once, on first insert), not the file's
// own mtime, so re-tagging an old file doesn't bump it to the top.
func (s *Store) ListRecentlyAdded(limit int) ([]Track, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		ORDER BY t.added_at DESC, t.id LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// RecordPlay logs a play event — called by the playback engine whenever a
// track starts playing, so ListRecentlyPlayed has something to work with.
func (s *Store) RecordPlay(trackID string) error {
	_, err := s.db.Exec(`INSERT INTO play_history (track_id, played_at) VALUES (?, ?)`, trackID, time.Now().Unix())
	return err
}

// ListRecentlyPlayed returns distinct tracks ordered by their most recent
// play, newest first.
func (s *Store) ListRecentlyPlayed(limit int) ([]Track, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM (SELECT track_id, MAX(played_at) AS played_at FROM play_history GROUP BY track_id) h
		JOIN tracks t ON t.id = h.track_id
		`+trackJoins+`
		ORDER BY h.played_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// TopGenreByHour returns the genre with the most plays, within the last
// `days` days, among plays that happened during any of the given hours of
// day (0-23, local time) — e.g. hours 5-10 for "morning". Used to
// personalize the home screen's AI quick links. Returns "" if there's no
// play history matching, rather than an error.
func (s *Store) TopGenreByHour(hours []int, days int) (string, int, error) {
	return s.topTrackFieldByHour("genre", -1, hours, days)
}

// TopArtistByHour is TopGenreByHour, grouped by artist instead of genre.
func (s *Store) TopArtistByHour(hours []int, days int) (string, int, error) {
	return s.topTrackFieldByHour("artist", -1, hours, days)
}

// TopGenreByWeekdayHour is TopGenreByHour narrowed to one weekday as well
// (0=Sunday..6=Saturday, matching SQLite's %w) — a tighter "same time on
// this day of the week" signal than the hour-only aggregate, meant to be
// run over a much shorter lookback.
func (s *Store) TopGenreByWeekdayHour(weekday int, hours []int, days int) (string, int, error) {
	return s.topTrackFieldByHour("genre", weekday, hours, days)
}

// topTrackFieldByHour groups play_history within the given lookback window
// and hours-of-day — optionally narrowed to one weekday (0=Sunday..6=
// Saturday; pass -1 for any day) — by the named tracks column and returns
// the most common non-empty value. field is always one of our own
// hardcoded column names (never user input), so it's safe to interpolate
// into the query.
func (s *Store) topTrackFieldByHour(field string, weekday int, hours []int, days int) (string, int, error) {
	if len(hours) == 0 {
		return "", 0, nil
	}
	placeholders := make([]string, len(hours))
	args := make([]any, 0, len(hours)+2)
	args = append(args, time.Now().AddDate(0, 0, -days).Unix())
	for i, h := range hours {
		placeholders[i] = "?"
		args = append(args, h)
	}
	weekdayClause := ""
	if weekday >= 0 {
		weekdayClause = "AND CAST(strftime('%w', h.played_at, 'unixepoch', 'localtime') AS INTEGER) = ?"
		args = append(args, weekday)
	}
	query := fmt.Sprintf(`
		SELECT t.%s, COUNT(*) AS c
		FROM play_history h
		JOIN tracks t ON t.id = h.track_id
		WHERE h.played_at >= ?
			AND CAST(strftime('%%H', h.played_at, 'unixepoch', 'localtime') AS INTEGER) IN (%s)
			%s
			AND t.%s IS NOT NULL AND t.%s != ''
		GROUP BY t.%s
		ORDER BY c DESC
		LIMIT 1
	`, field, strings.Join(placeholders, ","), weekdayClause, field, field, field)

	var value string
	var count int
	if err := s.db.QueryRow(query, args...).Scan(&value, &count); err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, err
	}
	return value, count, nil
}

// RecentPlayStreakGenre returns the genre with the most occurrences among
// the last `n` play events (most recent first, repeats included), plus
// how many of those plays it accounts for — a "you've been on a run of X"
// signal distinct from TopGenreByHour's longer, hour-of-day pattern.
func (s *Store) RecentPlayStreakGenre(n int) (string, int, error) {
	rows, err := s.db.Query(`
		SELECT t.genre
		FROM play_history h
		JOIN tracks t ON t.id = h.track_id
		WHERE t.genre IS NOT NULL AND t.genre != ''
		ORDER BY h.played_at DESC
		LIMIT ?
	`, n)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var genre string
		if err := rows.Scan(&genre); err != nil {
			return "", 0, err
		}
		counts[genre]++
	}
	if err := rows.Err(); err != nil {
		return "", 0, err
	}

	var top string
	var topCount int
	for genre, c := range counts {
		if c > topCount {
			top, topCount = genre, c
		}
	}
	return top, topCount, nil
}

// TopArtistOverall returns the artist with the most all-time plays.
func (s *Store) TopArtistOverall() (string, int, error) {
	var artist string
	var count int
	err := s.db.QueryRow(`
		SELECT t.artist, COUNT(*) AS c
		FROM play_history h
		JOIN tracks t ON t.id = h.track_id
		WHERE t.artist IS NOT NULL AND t.artist != ''
		GROUP BY t.artist
		ORDER BY c DESC
		LIMIT 1
	`).Scan(&artist, &count)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	return artist, count, err
}

// scanOptionalTrack scans a single-track query result where zero rows is
// an expected, non-error outcome (found=false) rather than sql.ErrNoRows
// bubbling up to the caller.
func scanOptionalTrack(row *sql.Row) (Track, bool, error) {
	t, err := scanTrack(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Track{}, false, nil
		}
		return Track{}, false, err
	}
	return t, true, nil
}

// StaleFavorite returns a favorited or highly-rated (4+) track that hasn't
// been played in the last `days` days — preferring one that's never been
// played at all — as a nudge back toward something the listener already
// likes but has drifted away from. found is false when nothing is
// favorited/rated that highly, or everything's been played recently.
func (s *Store) StaleFavorite(days int) (Track, bool, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	row := s.db.QueryRow(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		LEFT JOIN (SELECT track_id, MAX(played_at) AS last_played FROM play_history GROUP BY track_id) lp ON lp.track_id = t.id
		WHERE (f.track_id IS NOT NULL OR COALESCE(r.rating, 0) >= 4)
			AND (lp.last_played IS NULL OR lp.last_played < ?)
		ORDER BY (lp.last_played IS NULL) DESC, lp.last_played ASC
		LIMIT 1
	`, cutoff)
	return scanOptionalTrack(row)
}

// UnplayedTrackByArtist returns a track credited to the given artist that
// has never appeared in play_history — a "deep cut" suggestion for an
// artist the listener already plays a lot.
func (s *Store) UnplayedTrackByArtist(artist string) (Track, bool, error) {
	row := s.db.QueryRow(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.artist = ?
			AND NOT EXISTS (SELECT 1 FROM play_history h WHERE h.track_id = t.id)
		ORDER BY t.album_id, t.disc_no, t.track_no
		LIMIT 1
	`, artist)
	return scanOptionalTrack(row)
}

// UnplayedRecentlyAdded returns the most recently added track — added
// within the last `days` days — that has never been played.
func (s *Store) UnplayedRecentlyAdded(days int) (Track, bool, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	row := s.db.QueryRow(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.added_at >= ?
			AND NOT EXISTS (SELECT 1 FROM play_history h WHERE h.track_id = t.id)
		ORDER BY t.added_at DESC
		LIMIT 1
	`, cutoff)
	return scanOptionalTrack(row)
}

// ListGenres returns the distinct, non-empty genre tags present in the
// library, alphabetized.
func (s *Store) ListGenres() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT genre FROM tracks WHERE genre != '' ORDER BY genre COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}
	return genres, nil
}

// TracksByGenre returns every track tagged with the given genre (case-insensitive, exact match).
func (s *Store) TracksByGenre(genre string) ([]Track, error) {
	rows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.genre = ? COLLATE NOCASE
		ORDER BY t.artist, t.album, t.disc_no, t.track_no
	`, genre)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// TrackFilter combines the filters Search (free-text OR match) and
// TracksByGenre (exact genre only) each handle separately — a caller (the
// AI tool layer, mainly) can AND together a free-text query, genre, artist,
// and year range in one pass instead of intersecting several query results
// itself.
type TrackFilter struct {
	Query    string
	Genre    string
	Artist   string
	YearFrom int
	YearTo   int
	Limit    int
}

func (s *Store) FilterTracks(f TrackFilter) ([]Track, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var where []string
	var args []any
	if f.Query != "" {
		where = append(where, "(t.title LIKE ? OR t.artist LIKE ? OR t.album LIKE ?)")
		p := "%" + f.Query + "%"
		args = append(args, p, p, p)
	}
	if f.Genre != "" {
		where = append(where, "t.genre = ? COLLATE NOCASE")
		args = append(args, f.Genre)
	}
	if f.Artist != "" {
		where = append(where, "(t.artist LIKE ? OR t.album_artist LIKE ?)")
		p := "%" + f.Artist + "%"
		args = append(args, p, p)
	}
	if f.YearFrom > 0 {
		where = append(where, "t.year >= ?")
		args = append(args, f.YearFrom)
	}
	if f.YearTo > 0 {
		where = append(where, "t.year <= ?")
		args = append(args, f.YearTo)
	}

	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit)

	rows, err := s.db.Query(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		`+clause+`
		ORDER BY t.artist, t.year, t.album, t.disc_no, t.track_no
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// Stats is a whole-library summary of counts and totals.
func (s *Store) Stats() (Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(size), 0), COALESCE(SUM(duration_ms), 0) FROM tracks
	`).Scan(&st.Tracks, &st.TotalSize, &st.TotalDurationMS); err != nil {
		return Stats{}, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM albums`).Scan(&st.Albums); err != nil {
		return Stats{}, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT album_artist) FROM albums WHERE album_artist != ''`).Scan(&st.Artists); err != nil {
		return Stats{}, err
	}
	return st, nil
}

// SetFavorite marks (or unmarks) a track as a favorite.
func (s *Store) SetFavorite(trackID string, favorite bool) error {
	if !favorite {
		_, err := s.db.Exec(`DELETE FROM favorites WHERE track_id = ?`, trackID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO favorites (track_id, created_at) VALUES (?, ?)
		ON CONFLICT(track_id) DO NOTHING
	`, trackID, time.Now().Unix())
	return err
}

// ListFavorites returns favorited tracks, most recently favorited first.
func (s *Store) ListFavorites() ([]Track, error) {
	rows, err := s.db.Query(`
		SELECT ` + trackColumns + `
		FROM tracks t ` + trackJoins + `
		WHERE f.track_id IS NOT NULL
		ORDER BY f.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// SetRating sets a track's 1-5 rating; rating <= 0 clears it.
func (s *Store) SetRating(trackID string, rating int) error {
	if rating <= 0 {
		_, err := s.db.Exec(`DELETE FROM ratings WHERE track_id = ?`, trackID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO ratings (track_id, rating) VALUES (?, ?)
		ON CONFLICT(track_id) DO UPDATE SET rating = excluded.rating
	`, trackID, rating)
	return err
}

// AddToPlaylist appends a single track, creating the playlist if it doesn't exist yet.
func (s *Store) AddToPlaylist(name, trackID string) error {
	ids, _, err := s.GetPlaylist(name)
	if err != nil {
		return err
	}
	ids = append(ids, trackID)
	return s.SavePlaylist(name, ids)
}

// RemoveFromPlaylist removes every occurrence of trackID from the named playlist.
func (s *Store) RemoveFromPlaylist(name, trackID string) error {
	ids, found, err := s.GetPlaylist(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("playlist %s not found", name)
	}
	out := ids[:0:0]
	for _, id := range ids {
		if id != trackID {
			out = append(out, id)
		}
	}
	return s.SavePlaylist(name, out)
}

// RenamePlaylist renames a playlist in place.
func (s *Store) RenamePlaylist(oldName, newName string) error {
	res, err := s.db.Exec(`UPDATE playlists SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("playlist %s not found", oldName)
	}
	return nil
}

// ListAlbumsMissingArt returns albums with no cached art yet — candidates
// for the background enrichment pipeline to query external providers for.
func (s *Store) ListAlbumsMissingArt() ([]Album, error) {
	rows, err := s.db.Query(`
		SELECT id, name, album_artist, year, sort_key, track_count FROM albums
		WHERE art_hash IS NULL OR art_hash = ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumArtist, &a.Year, &a.SortKey, &a.TrackCount); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, nil
}

// SavePlaylist creates or overwrites a named, ordered playlist.
func (s *Store) SavePlaylist(name string, trackIDs []string) error {
	data, err := json.Marshal(trackIDs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO playlists (name, track_ids) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET track_ids = excluded.track_ids
	`, name, string(data))
	return err
}

func (s *Store) GetPlaylist(name string) ([]string, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT track_ids FROM playlists WHERE name = ?`, name).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, false, err
	}
	return ids, true, nil
}

func (s *Store) ListPlaylists() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM playlists ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, nil
}

func (s *Store) DeletePlaylist(name string) error {
	_, err := s.db.Exec(`DELETE FROM playlists WHERE name = ?`, name)
	return err
}

// SaveQueueState checkpoints the play queue — track order, which entry is
// current, and how far into it playback had gotten — so the engine can
// restore it on the next startup. Called on every queue change and
// periodically while playing; overwrites the single saved snapshot.
func (s *Store) SaveQueueState(trackIDs []string, position, positionMS int) error {
	data, err := json.Marshal(trackIDs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO queue_state (id, track_ids, position, position_ms) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET track_ids = excluded.track_ids, position = excluded.position, position_ms = excluded.position_ms
	`, string(data), position, positionMS)
	return err
}

// LoadQueueState returns the last saved queue snapshot, or a nil slice if
// none was ever saved (fresh database).
func (s *Store) LoadQueueState() (trackIDs []string, position, positionMS int, err error) {
	var raw string
	err = s.db.QueryRow(`SELECT track_ids, position, position_ms FROM queue_state WHERE id = 1`).Scan(&raw, &position, &positionMS)
	if err == sql.ErrNoRows {
		return nil, -1, 0, nil
	}
	if err != nil {
		return nil, -1, 0, err
	}
	if err := json.Unmarshal([]byte(raw), &trackIDs); err != nil {
		return nil, -1, 0, err
	}
	return trackIDs, position, positionMS, nil
}

func (s *Store) GetTrack(id string) (Track, bool, error) {
	row := s.db.QueryRow(`
		SELECT `+trackColumns+`
		FROM tracks t `+trackJoins+`
		WHERE t.id = ?
	`, id)
	t, err := scanTrack(row)
	if err == sql.ErrNoRows {
		return Track{}, false, nil
	}
	return t, err == nil, err
}

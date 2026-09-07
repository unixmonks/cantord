package library

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

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
	return &Store{db: db}, nil
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
		INSERT INTO tracks (id, path, title, artist, album, album_artist, album_id, track_no, disc_no, year, genre, duration_ms, codec, sample_rate, bit_depth, channels, size, mtime)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
			id=excluded.id, title=excluded.title, artist=excluded.artist, album=excluded.album,
			album_artist=excluded.album_artist, album_id=excluded.album_id, track_no=excluded.track_no,
			disc_no=excluded.disc_no, year=excluded.year, genre=excluded.genre, duration_ms=excluded.duration_ms,
			codec=excluded.codec, sample_rate=excluded.sample_rate, bit_depth=excluded.bit_depth,
			channels=excluded.channels, size=excluded.size, mtime=excluded.mtime
	`, t.ID, t.Path, t.Title, t.Artist, t.Album, albumArtist, albumID, t.TrackNo, t.DiscNo, t.Year, t.Genre,
		t.DurationMS, t.Codec, t.SampleRate, t.BitDepth, t.Channels, t.Size, t.MTime)
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

// SearchResult is the combined hit set for a query: matching albums and
// matching tracks, so a client can jump straight to either.
type SearchResult struct {
	Albums []Album `json:"albums"`
	Tracks []Track `json:"tracks"`
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

	albumRows, err := s.db.Query(`
		SELECT id, name, album_artist, year, sort_key, art_hash, track_count FROM albums
		WHERE name LIKE ? OR album_artist LIKE ?
		ORDER BY sort_key, id LIMIT ?
	`, pattern, pattern, limit)
	if err != nil {
		return SearchResult{}, err
	}
	defer albumRows.Close()

	var result SearchResult
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
		SELECT id, path, title, artist, album, album_artist, album_id, track_no, disc_no, year, genre,
		       duration_ms, codec, sample_rate, bit_depth, channels, size, mtime
		FROM tracks
		WHERE title LIKE ? OR artist LIKE ? OR album LIKE ? OR album_artist LIKE ?
		ORDER BY artist, album, disc_no, track_no LIMIT ?
	`, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return SearchResult{}, err
	}
	defer trackRows.Close()

	for trackRows.Next() {
		var t Track
		if err := trackRows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.AlbumID,
			&t.TrackNo, &t.DiscNo, &t.Year, &t.Genre, &t.DurationMS, &t.Codec, &t.SampleRate, &t.BitDepth,
			&t.Channels, &t.Size, &t.MTime); err != nil {
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
		SELECT id, path, title, artist, album, album_artist, album_id, track_no, disc_no, year, genre,
		       duration_ms, codec, sample_rate, bit_depth, channels, size, mtime
		FROM tracks WHERE album_id = ? ORDER BY disc_no, track_no, title
	`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.AlbumID,
			&t.TrackNo, &t.DiscNo, &t.Year, &t.Genre, &t.DurationMS, &t.Codec, &t.SampleRate, &t.BitDepth,
			&t.Channels, &t.Size, &t.MTime); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
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

func (s *Store) GetTrack(id string) (Track, bool, error) {
	var t Track
	err := s.db.QueryRow(`
		SELECT id, path, title, artist, album, album_artist, album_id, track_no, disc_no, year, genre,
		       duration_ms, codec, sample_rate, bit_depth, channels, size, mtime
		FROM tracks WHERE id = ?
	`, id).Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.AlbumID,
		&t.TrackNo, &t.DiscNo, &t.Year, &t.Genre, &t.DurationMS, &t.Codec, &t.SampleRate, &t.BitDepth,
		&t.Channels, &t.Size, &t.MTime)
	if err == sql.ErrNoRows {
		return Track{}, false, nil
	}
	return t, err == nil, err
}

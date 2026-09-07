package library

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"cantord/internal/art"
	"cantord/internal/events"
)

var audioExtensions = map[string]bool{
	".flac": true, ".mp3": true, ".m4a": true, ".mp4": true, ".alac": true,
	".ogg": true, ".oga": true, ".opus": true, ".wav": true, ".aiff": true,
	".aif": true, ".dsf": true, ".dff": true, ".ape": true, ".wv": true,
	".wma": true,
}

type Scanner struct {
	store       *Store
	artStore    *art.Store
	bus         *events.Bus
	ffprobePath string
	musicDirs   []string
}

func NewScanner(store *Store, artStore *art.Store, bus *events.Bus, ffprobePath string, musicDirs []string) *Scanner {
	return &Scanner{store: store, artStore: artStore, bus: bus, ffprobePath: ffprobePath, musicDirs: musicDirs}
}

// Scan walks the configured music directories, upserting any new or
// changed track into the library store (unchanged files, by size+mtime,
// are skipped without re-reading tags or re-probing) and removing rows for
// files that no longer exist.
func (sc *Scanner) Scan(ctx context.Context) error {
	slog.Info("scan: starting", "dirs", sc.musicDirs)
	seen := map[string]bool{}
	var added, skipped, failed int

	for _, root := range sc.musicDirs {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				slog.Warn("scan: walk error", "path", path, "err", err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !audioExtensions[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			seen[path] = true

			info, err := d.Info()
			if err != nil {
				failed++
				return nil
			}
			size := info.Size()
			mtime := info.ModTime().Unix()

			if pSize, pMTime, found := sc.store.TrackFingerprint(path); found && pSize == size && pMTime == mtime {
				skipped++
				return nil
			}

			if err := sc.processFile(ctx, path, size, mtime); err != nil {
				slog.Warn("scan: processing file", "path", path, "err", err)
				failed++
				return nil
			}
			added++
			return nil
		})
		if err != nil {
			return err
		}
	}

	removed, err := sc.store.RemoveMissing(seen)
	if err != nil {
		slog.Warn("scan: removing missing tracks", "err", err)
	}

	slog.Info("scan: complete", "added_or_updated", added, "skipped_unchanged", skipped, "removed", removed, "failed", failed)
	sc.bus.Publish(events.Event{Type: "library_changed", Data: map[string]int{
		"added_or_updated": added, "removed": removed, "failed": failed,
	}})
	return nil
}

func (sc *Scanner) processFile(ctx context.Context, path string, size, mtime int64) error {
	tagInfo, err := ReadTags(path)
	if err != nil {
		return err
	}
	stream, err := Probe(ctx, sc.ffprobePath, path)
	if err != nil {
		// Missing stream info shouldn't drop the track from the library —
		// tags alone are enough to list/queue it.
		slog.Warn("scan: ffprobe failed, indexing with tags only", "path", path, "err", err)
	}

	title := tagInfo.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	track := Track{
		Path:        path,
		Title:       title,
		Artist:      tagInfo.Artist,
		Album:       tagInfo.Album,
		AlbumArtist: tagInfo.AlbumArtist,
		TrackNo:     tagInfo.TrackNo,
		DiscNo:      tagInfo.DiscNo,
		Year:        tagInfo.Year,
		Genre:       tagInfo.Genre,
		DurationMS:  stream.DurationMS,
		Codec:       stream.Codec,
		SampleRate:  stream.SampleRate,
		BitDepth:    stream.BitDepth,
		Channels:    stream.Channels,
		Size:        size,
		MTime:       mtime,
	}

	albumID, err := sc.store.UpsertTrack(track)
	if err != nil {
		return err
	}

	if len(tagInfo.ArtBytes) > 0 {
		hash, err := sc.artStore.Put(tagInfo.ArtBytes)
		if err != nil {
			slog.Warn("scan: caching embedded art", "path", path, "err", err)
		} else if err := sc.store.SetAlbumArtIfEmpty(albumID, hash); err != nil {
			slog.Warn("scan: saving embedded art hash", "path", path, "err", err)
		}
	}

	return nil
}

package library

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cantord/internal/art"
	"cantord/internal/events"
)

var audioExtensions = map[string]bool{
	".flac": true, ".mp3": true, ".m4a": true, ".mp4": true, ".alac": true,
	".ogg": true, ".oga": true, ".opus": true, ".wav": true, ".aiff": true,
	".aif": true, ".dsf": true, ".dff": true, ".ape": true, ".wv": true,
	".wma": true,
}

// ScanProgress is a snapshot of an in-progress (or last completed) scan,
// polled via GET /api/library/scan/status and pushed as the scan_progress
// SSE event.
type ScanProgress struct {
	Running        bool   `json:"running"`
	Total          int    `json:"total"`
	Processed      int    `json:"processed"`
	AddedOrUpdated int    `json:"added_or_updated"`
	Skipped        int    `json:"skipped_unchanged"`
	Removed        int    `json:"removed"`
	Failed         int    `json:"failed"`
	CurrentPath    string `json:"current_path,omitempty"`
}

type Scanner struct {
	store       *Store
	artStore    *art.Store
	bus         *events.Bus
	ffprobePath string
	musicDirs   []string

	mu       sync.Mutex
	progress ScanProgress
}

func NewScanner(store *Store, artStore *art.Store, bus *events.Bus, ffprobePath string, musicDirs []string) *Scanner {
	return &Scanner{store: store, artStore: artStore, bus: bus, ffprobePath: ffprobePath, musicDirs: musicDirs}
}

// Progress returns a snapshot of the current (or most recently finished) scan.
func (sc *Scanner) Progress() ScanProgress {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.progress
}

func (sc *Scanner) setProgress(mutate func(*ScanProgress)) ScanProgress {
	sc.mu.Lock()
	mutate(&sc.progress)
	snapshot := sc.progress
	sc.mu.Unlock()
	return snapshot
}

// countAudioFiles is a fast pass (stat only, no tags/ffprobe) so the real
// scan can report "processed N of Total" instead of just a running count.
func (sc *Scanner) countAudioFiles() int {
	total := 0
	for _, root := range sc.musicDirs {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if audioExtensions[strings.ToLower(filepath.Ext(path))] {
				total++
			}
			return nil
		})
	}
	return total
}

// Scan walks the configured music directories, upserting any new or
// changed track into the library store (unchanged files, by size+mtime,
// are skipped without re-reading tags or re-probing) and removing rows for
// files that no longer exist. Progress is logged periodically, published
// on the event bus as scan_progress, and available via Progress().
func (sc *Scanner) Scan(ctx context.Context) error {
	total := sc.countAudioFiles()
	slog.Info("scan: starting", "dirs", sc.musicDirs, "total_files", total)
	sc.setProgress(func(p *ScanProgress) {
		*p = ScanProgress{Running: true, Total: total}
	})

	seen := map[string]bool{}
	lastReport := time.Now()

	report := func(force bool) {
		if !force && time.Since(lastReport) < 500*time.Millisecond {
			return
		}
		lastReport = time.Now()
		snap := sc.Progress()
		slog.Info("scan: progress", "processed", snap.Processed, "total", snap.Total,
			"added_or_updated", snap.AddedOrUpdated, "skipped_unchanged", snap.Skipped, "failed", snap.Failed)
		sc.bus.Publish(events.Event{Type: "scan_progress", Data: snap})
	}

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
				sc.setProgress(func(p *ScanProgress) { p.Processed++; p.Failed++; p.CurrentPath = path })
				report(false)
				return nil
			}
			size := info.Size()
			mtime := info.ModTime().Unix()

			if pSize, pMTime, found := sc.store.TrackFingerprint(path); found && pSize == size && pMTime == mtime {
				sc.setProgress(func(p *ScanProgress) { p.Processed++; p.Skipped++; p.CurrentPath = path })
				report(false)
				return nil
			}

			if err := sc.processFile(ctx, path, size, mtime); err != nil {
				slog.Warn("scan: processing file", "path", path, "err", err)
				sc.setProgress(func(p *ScanProgress) { p.Processed++; p.Failed++; p.CurrentPath = path })
				report(false)
				return nil
			}
			sc.setProgress(func(p *ScanProgress) { p.Processed++; p.AddedOrUpdated++; p.CurrentPath = path })
			report(false)
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

	final := sc.setProgress(func(p *ScanProgress) {
		p.Running = false
		p.Removed = removed
		p.CurrentPath = ""
	})

	slog.Info("scan: complete", "added_or_updated", final.AddedOrUpdated, "skipped_unchanged", final.Skipped,
		"removed", final.Removed, "failed", final.Failed)
	sc.bus.Publish(events.Event{Type: "library_changed", Data: final})
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

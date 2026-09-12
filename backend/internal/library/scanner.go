package library

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

type Scanner struct {
	store       *Store
	artStore    *art.Store
	bus         *events.Bus
	ffprobePath string
	musicDirs   []string

	running atomic.Bool

	mu       sync.Mutex
	progress ScanProgress

	rootMu        sync.Mutex
	rootReachable map[string]bool // last known reachability per root, for transition-only logging
}

func NewScanner(store *Store, artStore *art.Store, bus *events.Bus, ffprobePath string, musicDirs []string) *Scanner {
	return &Scanner{
		store: store, artStore: artStore, bus: bus, ffprobePath: ffprobePath, musicDirs: musicDirs,
		rootReachable: make(map[string]bool),
	}
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
// are skipped without re-reading tags or re-probing). For each root whose
// walk completes without error, tracks it didn't see get marked
// unavailable and tracks it did see (again) get marked available —
// scanning never deletes anything, so an unreachable NFS mount (which
// surfaces as a walk error) leaves that root's library entries untouched
// instead of wiping them. Progress is logged periodically, published on
// the event bus as scan_progress, and available via Progress().
func (sc *Scanner) Scan(ctx context.Context) error {
	if !sc.running.CompareAndSwap(false, true) {
		slog.Info("scan: already running, skipping")
		return nil
	}
	defer sc.running.Store(false)

	total := sc.countAudioFiles()
	slog.Info("scan: starting", "dirs", sc.musicDirs, "total_files", total)
	sc.setProgress(func(p *ScanProgress) {
		*p = ScanProgress{Running: true, Total: total}
	})

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

	var totalMarkedMissing, totalMarkedAvailable int

	for _, root := range sc.musicDirs {
		seen := map[string]bool{}
		walkErr := false

		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Covers both "root doesn't exist" (e.g. an NFS mount that
				// went away — WalkDir invokes this once for the root path
				// itself) and a subtree that errors out partway through.
				// Either way we can't trust this root's seen-set, so it
				// must not be used to mark anything unavailable below.
				walkErr = true
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

		reachable := !walkErr
		sc.noteRootReachability(root, reachable)
		if !reachable {
			continue
		}

		markedMissing, markedAvailable, err := sc.store.SetAvailability(root, seen)
		if err != nil {
			slog.Warn("scan: reconciling availability", "root", root, "err", err)
			continue
		}
		totalMarkedMissing += markedMissing
		totalMarkedAvailable += markedAvailable
	}

	unavailable, err := sc.store.CountUnavailable()
	if err != nil {
		slog.Warn("scan: counting unavailable tracks", "err", err)
	}

	final := sc.setProgress(func(p *ScanProgress) {
		p.Running = false
		p.MarkedMissing = totalMarkedMissing
		p.MarkedAvailable = totalMarkedAvailable
		p.Unavailable = unavailable
		p.CurrentPath = ""
	})

	slog.Info("scan: complete", "added_or_updated", final.AddedOrUpdated, "skipped_unchanged", final.Skipped,
		"marked_missing", final.MarkedMissing, "marked_available", final.MarkedAvailable,
		"unavailable", final.Unavailable, "failed", final.Failed)
	sc.bus.Publish(events.Event{Type: "library_changed", Data: final})
	return nil
}

// noteRootReachability logs (once, on transition) when a music dir becomes
// unreachable or comes back, and publishes it on the event bus so a client
// can show something more useful than tracks silently going quiet.
func (sc *Scanner) noteRootReachability(root string, reachable bool) {
	sc.rootMu.Lock()
	prev, known := sc.rootReachable[root]
	sc.rootReachable[root] = reachable
	sc.rootMu.Unlock()

	if known && prev == reachable {
		return
	}
	if reachable {
		slog.Info("scan: root reachable", "root", root)
	} else {
		slog.Warn("scan: root unreachable, leaving its library entries as-is", "root", root)
	}
	sc.bus.Publish(events.Event{Type: "root_reachability", Data: map[string]any{"root": root, "reachable": reachable}})
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

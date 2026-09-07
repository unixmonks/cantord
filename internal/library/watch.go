package library

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch watches the scanner's music directories (recursively, and picking
// up newly-created subdirectories) and triggers a debounced rescan on any
// change. fsnotify wraps inotify on Linux and kqueue on FreeBSD, so this is
// one code path for both targets.
func (sc *Scanner) Watch(ctx context.Context, debounce time.Duration) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	for _, root := range sc.musicDirs {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if err := watcher.Add(path); err != nil {
				slog.Warn("watch: adding directory", "path", path, "err", err)
			}
			return nil
		})
	}

	var timer *time.Timer
	rescan := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil

		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = watcher.Add(ev.Name)
				}
			}
			if timer == nil {
				timer = time.AfterFunc(debounce, func() {
					select {
					case rescan <- struct{}{}:
					default:
					}
				})
			} else {
				timer.Reset(debounce)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watch: fsnotify error", "err", err)

		case <-rescan:
			if err := sc.Scan(ctx); err != nil {
				slog.Warn("watch: rescan failed", "err", err)
			}
		}
	}
}

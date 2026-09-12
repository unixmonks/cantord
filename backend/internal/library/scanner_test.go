package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cantord/internal/art"
	"cantord/internal/events"
)

func newTestScanner(t *testing.T, musicDirs []string) (*Scanner, *Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	artStore, err := art.New(filepath.Join(dir, "art"))
	if err != nil {
		t.Fatalf("opening art store: %v", err)
	}

	bus := events.NewBus()
	sc := NewScanner(store, artStore, bus, "cantord-test-nonexistent-ffprobe", musicDirs)
	return sc, store
}

// TestScanUnreachableRootDoesNotDeleteLibrary is a regression test for the
// bug this feature was built to fix: scanning a root that doesn't exist
// (e.g. an unmounted NFS share) must leave existing library rows alone
// instead of wiping them, because filepath.WalkDir reports the missing root
// as a walk error rather than an empty directory.
func TestScanUnreachableRootDoesNotDeleteLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	sc, store := newTestScanner(t, []string{root})

	trackPath := filepath.Join(root, "song.flac")
	if _, err := store.UpsertTrack(Track{Path: trackPath, Title: "Song", Artist: "Artist", Album: "Album"}); err != nil {
		t.Fatalf("seeding track: %v", err)
	}

	if err := sc.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	tr, found, err := store.GetTrack(hashHex(trackPath))
	if err != nil {
		t.Fatalf("get track: %v", err)
	}
	if !found {
		t.Fatal("track was deleted by a scan of an unreachable root")
	}
	if !tr.Available {
		t.Fatal("an unreachable root must not change availability at all")
	}
}

// TestScanMarksMissingThenAvailableAgain covers the reconnect path: once a
// root can be walked cleanly, a track that isn't there gets marked
// unavailable (not deleted), and gets marked available again once seen.
func TestScanMarksMissingThenAvailableAgain(t *testing.T) {
	root := t.TempDir()
	sc, store := newTestScanner(t, []string{root})

	trackPath := filepath.Join(root, "song.flac")
	if _, err := store.UpsertTrack(Track{Path: trackPath, Title: "Song", Artist: "Artist", Album: "Album"}); err != nil {
		t.Fatalf("seeding track: %v", err)
	}

	// The file doesn't exist on disk yet: a clean scan of an existing but
	// empty root should mark the track unavailable, never delete it.
	if err := sc.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	tr, found, err := store.GetTrack(hashHex(trackPath))
	if err != nil || !found {
		t.Fatalf("get track: found=%v err=%v", found, err)
	}
	if tr.Available {
		t.Fatal("track should be unavailable once its root was scanned cleanly and it wasn't found")
	}
	if tr.MissingSince == 0 {
		t.Fatal("expected missing_since to be set")
	}

	// The file reappears (mount recovers, file restored) — availability
	// should flip back without needing tag/probe to succeed.
	if err := os.WriteFile(trackPath, []byte("not a real flac file"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	if err := sc.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	tr, found, err = store.GetTrack(hashHex(trackPath))
	if err != nil || !found {
		t.Fatalf("get track: found=%v err=%v", found, err)
	}
	if !tr.Available {
		t.Fatal("track should be available again now that the file is back")
	}
}

// TestPruneUnavailable checks the explicit, manual cleanup path: nothing
// gets deleted until it's called.
func TestPruneUnavailable(t *testing.T) {
	root := t.TempDir()
	sc, store := newTestScanner(t, []string{root})

	trackPath := filepath.Join(root, "song.flac")
	if _, err := store.UpsertTrack(Track{Path: trackPath, Title: "Song", Artist: "Artist", Album: "Album"}); err != nil {
		t.Fatalf("seeding track: %v", err)
	}
	if err := sc.Scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	removed, err := store.PruneUnavailable()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if _, found, err := store.GetTrack(hashHex(trackPath)); err != nil || found {
		t.Fatalf("expected track to be gone: found=%v err=%v", found, err)
	}
}

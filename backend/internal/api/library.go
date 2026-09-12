package api

import (
	"context"
	"log/slog"
	"net/http"

	"cantord/internal/events"
)

// triggerScan kicks off a rescan in the background; progress/completion is
// reported via the library_changed event on /api/events rather than making
// the caller wait on a potentially slow filesystem walk.
func (s *Server) triggerScan(w http.ResponseWriter, r *http.Request) {
	slog.Info("api: scan triggered")
	go func() {
		if err := s.scanner.Scan(context.Background()); err != nil {
			slog.Error("api: triggered scan failed", "err", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

// scanStatus is a poll-friendly alternative to the scan_progress SSE event
// — {"running":false} once a scan finishes or before the first one runs.
func (s *Server) scanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.scanner.Progress())
}

// pruneLibrary permanently deletes tracks a scan has marked unavailable
// (their music dir was reachable and just didn't have the file anymore).
// Scanning alone never deletes — this is the explicit, user-initiated
// cleanup step for content that's actually gone rather than a storage
// hiccup.
func (s *Server) pruneLibrary(w http.ResponseWriter, r *http.Request) {
	removed, err := s.lib.PruneUnavailable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: pruned unavailable tracks", "removed", removed)
	s.bus.Publish(events.Event{Type: "library_changed", Data: map[string]any{"pruned": removed}})
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}

func (s *Server) libraryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.lib.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// Version is cantord's build version — no release/versioning scheme exists
// yet, so this is a placeholder clients can still poll to detect upgrades.
const Version = "0.1.0"

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": Version})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

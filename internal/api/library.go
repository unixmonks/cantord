package api

import (
	"context"
	"log/slog"
	"net/http"
)

// triggerScan kicks off a rescan in the background; progress/completion is
// reported via the library_changed event on /api/events rather than making
// the caller wait on a potentially slow filesystem walk.
func (s *Server) triggerScan(w http.ResponseWriter, r *http.Request) {
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

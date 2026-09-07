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

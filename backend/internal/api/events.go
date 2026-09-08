package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// streamEvents is an SSE feed of daemon events (status ticks, queue
// changes, art becoming available) so a frontend never has to poll.
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := s.bus.Subscribe()
	defer cancel()

	// Prime the connection with a current snapshot so the client doesn't
	// wait for the next tick to render anything.
	fmt.Fprintf(w, "event: status\ndata: %s\n\n", mustJSON(s.engine.Status()))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, mustJSON(ev.Data))
			flusher.Flush()
		}
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

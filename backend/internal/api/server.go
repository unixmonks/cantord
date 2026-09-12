// Package api exposes cantord's HTTP/JSON control surface: library
// browsing (paginated, alphabetical), a content-addressed art endpoint
// meant to be hit directly from <img> tags, queue/playback control, saved
// playlists, and an SSE event stream so clients don't have to poll.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"cantord/internal/ai"
	"cantord/internal/art"
	"cantord/internal/events"
	"cantord/internal/library"
	"cantord/internal/playback"
)

type Server struct {
	lib      *library.Store
	artStore *art.Store
	engine   *playback.Engine
	bus      *events.Bus
	scanner  *library.Scanner
	ai       *ai.Service
}

func New(lib *library.Store, artStore *art.Store, engine *playback.Engine, bus *events.Bus, scanner *library.Scanner, aiService *ai.Service) *Server {
	return &Server{lib: lib, artStore: artStore, engine: engine, bus: bus, scanner: scanner, ai: aiService}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("GET /api/artists", s.listArtists)
	mux.HandleFunc("GET /api/albums", s.listAlbums)
	mux.HandleFunc("GET /api/albums/index", s.albumIndex)
	mux.HandleFunc("GET /api/albums/{id}", s.getAlbum)
	mux.HandleFunc("GET /api/albums/{id}/tracks", s.listAlbumTracks)
	mux.HandleFunc("GET /api/tracks/recent", s.recentlyAdded)
	mux.HandleFunc("GET /api/tracks/recently-played", s.recentlyPlayed)
	mux.HandleFunc("GET /api/tracks/favorites", s.listFavorites)
	mux.HandleFunc("GET /api/tracks/{id}", s.getTrack)
	mux.HandleFunc("POST /api/tracks/{id}/favorite", s.setFavorite)
	mux.HandleFunc("POST /api/tracks/{id}/rating", s.setRating)

	mux.HandleFunc("GET /api/genres", s.listGenres)
	mux.HandleFunc("GET /api/genres/{genre}/tracks", s.tracksByGenre)

	mux.HandleFunc("GET /art/{hash}", s.getArt)

	mux.HandleFunc("GET /api/queue", s.getQueue)
	mux.HandleFunc("POST /api/queue", s.enqueue)
	mux.HandleFunc("POST /api/queue/next", s.playNext)
	mux.HandleFunc("POST /api/queue/move", s.moveQueue)
	mux.HandleFunc("POST /api/queue/clear", s.clearQueue)
	mux.HandleFunc("DELETE /api/queue/{index}", s.removeFromQueue)
	mux.HandleFunc("POST /api/queue/{index}/play", s.playIndex)

	mux.HandleFunc("GET /api/status", s.getStatus)
	mux.HandleFunc("POST /api/playback/play", s.simple(func() error { return s.engine.Play() }))
	mux.HandleFunc("POST /api/playback/pause", s.simple(func() error { return s.engine.Pause() }))
	mux.HandleFunc("POST /api/playback/stop", s.simple(func() error { return s.engine.Stop() }))
	mux.HandleFunc("POST /api/playback/next", s.simple(func() error { return s.engine.Next() }))
	mux.HandleFunc("POST /api/playback/previous", s.simple(func() error { return s.engine.Previous() }))
	mux.HandleFunc("POST /api/playback/seek", s.seek)
	mux.HandleFunc("POST /api/playback/volume", s.setVolume)
	mux.HandleFunc("POST /api/playback/mute", s.setMute)
	mux.HandleFunc("POST /api/playback/shuffle", s.setShuffle)
	mux.HandleFunc("POST /api/playback/repeat", s.setRepeat)

	mux.HandleFunc("GET /api/playlists", s.listPlaylists)
	mux.HandleFunc("POST /api/playlists", s.savePlaylist)
	mux.HandleFunc("GET /api/playlists/{name}", s.getPlaylist)
	mux.HandleFunc("DELETE /api/playlists/{name}", s.deletePlaylist)
	mux.HandleFunc("POST /api/playlists/{name}/tracks", s.addToPlaylist)
	mux.HandleFunc("DELETE /api/playlists/{name}/tracks/{track_id}", s.removeFromPlaylist)
	mux.HandleFunc("POST /api/playlists/{name}/rename", s.renamePlaylist)

	mux.HandleFunc("GET /api/ai/status", s.aiStatus)
	mux.HandleFunc("GET /api/ai/suggestions", s.aiSuggestions)
	mux.HandleFunc("GET /api/ai/conversations", s.listAiConversations)
	mux.HandleFunc("GET /api/ai/conversations/{id}", s.getAiConversation)
	mux.HandleFunc("DELETE /api/ai/conversations/{id}", s.deleteAiConversation)
	mux.HandleFunc("POST /api/ai/chat", s.aiChat)

	mux.HandleFunc("POST /api/library/scan", s.triggerScan)
	mux.HandleFunc("GET /api/library/scan/status", s.scanStatus)
	mux.HandleFunc("POST /api/library/prune", s.pruneLibrary)
	mux.HandleFunc("GET /api/library/stats", s.libraryStats)
	mux.HandleFunc("GET /api/events", s.streamEvents)

	mux.HandleFunc("GET /api/version", s.version)
	mux.HandleFunc("GET /api/health", s.health)

	return withCORS(withLogging(mux))
}

func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("http", "method", r.Method, "path", r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

// withCORS allows a browser-based frontend to talk to this API from another
// origin (e.g. a separately-hosted web UI, or a dev server on another port).
// There's no auth/cookies to protect, so reflecting the origin is safe.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// simple wraps a no-request-body, no-response-body playback action.
func (s *Server) simple(fn func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

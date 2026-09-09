package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

func trackLimit(r *http.Request, def int) int {
	limit := def
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return limit
}

func (s *Server) recentlyAdded(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.lib.ListRecentlyAdded(trackLimit(r, 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) recentlyPlayed(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.lib.ListRecentlyPlayed(trackLimit(r, 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) listFavorites(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.lib.ListFavorites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) setFavorite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Favorite bool `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id := r.PathValue("id")
	if err := s.lib.SetFavorite(id, body.Favorite); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: set favorite", "track_id", id, "favorite", body.Favorite)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rating int `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Rating < 0 || body.Rating > 5 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("rating must be 0-5"))
		return
	}
	id := r.PathValue("id")
	if err := s.lib.SetRating(id, body.Rating); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: set rating", "track_id", id, "rating", body.Rating)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := s.lib.ListGenres()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, genres)
}

func (s *Server) tracksByGenre(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.lib.TracksByGenre(r.PathValue("genre"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

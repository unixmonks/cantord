package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

func (s *Server) listPlaylists(w http.ResponseWriter, r *http.Request) {
	names, err := s.lib.ListPlaylists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *Server) savePlaylist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string   `json:"name"`
		TrackIDs []string `json:"track_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if err := s.lib.SavePlaylist(body.Name, body.TrackIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: save playlist", "name", body.Name, "tracks", len(body.TrackIDs))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getPlaylist(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ids, found, err := s.lib.GetPlaylist(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("playlist %s not found", name))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "track_ids": ids})
}

func (s *Server) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.lib.DeletePlaylist(name); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: delete playlist", "name", name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addToPlaylist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrackID string `json:"track_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.TrackID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("track_id is required"))
		return
	}
	name := r.PathValue("name")
	if err := s.lib.AddToPlaylist(name, body.TrackID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("api: add to playlist", "name", name, "track_id", body.TrackID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeFromPlaylist(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	trackID := r.PathValue("track_id")
	if err := s.lib.RemoveFromPlaylist(name, trackID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	slog.Info("api: remove from playlist", "name", name, "track_id", trackID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) renamePlaylist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.NewName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("new_name is required"))
		return
	}
	oldName := r.PathValue("name")
	if err := s.lib.RenamePlaylist(oldName, body.NewName); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	slog.Info("api: rename playlist", "from", oldName, "to", body.NewName)
	w.WriteHeader(http.StatusNoContent)
}

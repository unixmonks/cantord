package api

import (
	"encoding/json"
	"fmt"
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
	if err := s.lib.DeletePlaylist(r.PathValue("name")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

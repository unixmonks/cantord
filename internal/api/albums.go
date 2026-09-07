package api

import (
	"fmt"
	"net/http"
	"strconv"
)

func (s *Server) listAlbums(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	page, err := s.lib.ListAlbums(r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) listArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := s.lib.ListArtists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, artists)
}

func (s *Server) albumIndex(w http.ResponseWriter, r *http.Request) {
	index, err := s.lib.AlphabetIndex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, index)
}

func (s *Server) getAlbum(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	album, found, err := s.lib.GetAlbum(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("album %s not found", id))
		return
	}
	writeJSON(w, http.StatusOK, album)
}

func (s *Server) listAlbumTracks(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.lib.ListTracksForAlbum(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("q is required"))
		return
	}
	limit := 25
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	result, err := s.lib.Search(query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getTrack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	track, found, err := s.lib.GetTrack(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("track %s not found", id))
		return
	}
	writeJSON(w, http.StatusOK, track)
}

func (s *Server) getArt(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	size := r.URL.Query().Get("size")
	if size == "" {
		size = "thumb"
	}
	data, contentType, err := s.artStore.Open(hash, size)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("art %s not found", hash))
		return
	}
	// Content-addressed by hash: the same URL always returns the same
	// bytes, so clients (and browsers) can cache it forever.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", hash)
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

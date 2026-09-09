package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"cantord/internal/ai"
)

func (s *Server) aiStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": s.ai.Configured(),
		"provider":   "anthropic",
		"model":      s.ai.Model(),
	})
}

func (s *Server) listAiConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := s.lib.ListAiConversations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, convs)
}

func (s *Server) getAiConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	exists, err := s.lib.AiConversationExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, fmt.Errorf("conversation %s not found", id))
		return
	}
	messages, err := s.lib.ListAiMessages(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "messages": messages})
}

func (s *Server) deleteAiConversation(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.DeleteAiConversation(r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// aiChat runs one chat turn and streams the result back as SSE — same
// framing as /api/events, but scoped to this single request rather than
// the daemon-wide bus, since a POST body is needed and EventSource can't
// send one. The frontend reads this with fetch + a stream reader instead
// of EventSource.
func (s *Server) aiChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConversationID string `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Message) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("message is required"))
		return
	}
	if !s.ai.Configured() {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("AI assistant is not configured (set CANTORD_AI_API_KEY and restart cantord)"))
		return
	}

	convID := body.ConversationID
	if convID == "" {
		id, err := s.lib.CreateAiConversation()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		convID = id
	} else if exists, err := s.lib.AiConversationExists(convID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if !exists {
		writeError(w, http.StatusNotFound, fmt.Errorf("conversation %s not found", convID))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "event: conversation\ndata: %s\n\n", mustJSON(map[string]string{"conversation_id": convID}))
	flusher.Flush()

	err := s.ai.Chat(r.Context(), convID, body.Message, func(ev ai.Event) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, mustJSON(ev.Data))
		flusher.Flush()
	})
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", mustJSON(map[string]string{"error": err.Error()}))
		flusher.Flush()
	}
}

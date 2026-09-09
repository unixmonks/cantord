// Package ai wires an LLM into cantord as a tool-using assistant: it can
// search the library and act on it (build a queue, save a playlist)
// through the same operations the HTTP API already exposes, nothing more.
package ai

import (
	"context"
	"encoding/json"
)

// Event is one item in a chat turn's stream, forwarded to the HTTP layer
// as an SSE frame. Type is one of: text_delta, tool_call, tool_result, done, error.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// EventFunc receives stream events as a chat turn runs.
type EventFunc func(Event)

// trackSummary is the compact shape tool results hand back to the model —
// a full library.Track carries path/codec/bitrate fields that only waste
// tokens here.
type trackSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Year       int    `json:"year"`
	Genre      string `json:"genre"`
	DurationMS int    `json:"duration_ms"`
	Favorite   bool   `json:"favorite"`
	Rating     int    `json:"rating"`
}

// ToolResult is the JSON handed back to the model as a tool_result, and
// also what the API layer forwards to the frontend as the tool_result
// event's Data — the frontend renders TrackIDs.length / PlaylistName
// straight into an action card instead of parsing free text.
type ToolResult struct {
	Message      string   `json:"message"`
	Tracks       []any    `json:"tracks,omitempty"`
	TrackIDs     []string `json:"track_ids,omitempty"`
	PlaylistName string   `json:"playlist_name,omitempty"`
}

func (r ToolResult) json() string {
	b, err := json.Marshal(r)
	if err != nil {
		return r.Message
	}
	return string(b)
}

// Tool is one capability exposed to the model. InputSchema is the JSON
// Schema "properties" object (per-field types/descriptions); Required
// lists which of those properties must be present.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Required    []string
	Run         func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error)
}

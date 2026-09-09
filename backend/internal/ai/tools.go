package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cantord/internal/library"
	"cantord/internal/playback"
)

// Deps are the daemon components a tool handler is allowed to touch — the
// same library/engine operations the HTTP API already exposes, so the
// model can't do anything a user couldn't already do through the UI.
type Deps struct {
	Lib    *library.Store
	Engine *playback.Engine
}

func summarize(tracks []library.Track) []any {
	out := make([]any, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, trackSummary{
			ID: t.ID, Title: t.Title, Artist: t.Artist, Album: t.Album,
			Year: t.Year, Genre: t.Genre, DurationMS: t.DurationMS,
			Favorite: t.Favorite, Rating: t.Rating,
		})
	}
	return out
}

func tools() []Tool {
	return []Tool{
		{
			Name: "search_library",
			Description: "Search the user's actual music library for tracks. Combine a free-text " +
				"query (matches title/artist/album) with an exact genre and/or a year range to narrow " +
				"results. Genre tagging in real libraries is inconsistent, so when a genre filter returns " +
				"few or no results, retry with just a text query built from artist/song names you know fit " +
				"the request instead of guessing at genre spellings. Only tracks returned here actually " +
				"exist in the library — never queue or save a track_id you didn't get from this tool.",
			InputSchema: map[string]any{
				"query":     map[string]any{"type": "string", "description": "Free-text match against title, artist, and album."},
				"artist":    map[string]any{"type": "string", "description": "Filter to this artist or album artist (substring match)."},
				"genre":     map[string]any{"type": "string", "description": "Exact genre tag, e.g. \"Grunge\". Call list_genres first if unsure what's in use."},
				"year_from": map[string]any{"type": "integer", "description": "Earliest release year, inclusive."},
				"year_to":   map[string]any{"type": "integer", "description": "Latest release year, inclusive."},
				"limit":     map[string]any{"type": "integer", "description": "Max results, default 40, max 200."},
			},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				var args struct {
					Query    string `json:"query"`
					Artist   string `json:"artist"`
					Genre    string `json:"genre"`
					YearFrom int    `json:"year_from"`
					YearTo   int    `json:"year_to"`
					Limit    int    `json:"limit"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return ToolResult{}, err
				}
				tracks, err := deps.Lib.FilterTracks(library.TrackFilter{
					Query: args.Query, Artist: args.Artist, Genre: args.Genre,
					YearFrom: args.YearFrom, YearTo: args.YearTo, Limit: args.Limit,
				})
				if err != nil {
					return ToolResult{}, err
				}
				return ToolResult{
					Message: fmt.Sprintf("Found %d matching track(s).", len(tracks)),
					Tracks:  summarize(tracks),
				}, nil
			},
		},
		{
			Name:        "list_genres",
			Description: "List the exact genre tags actually present in the user's library, so search_library's genre filter matches something real instead of guessing.",
			InputSchema: map[string]any{},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				genres, err := deps.Lib.ListGenres()
				if err != nil {
					return ToolResult{}, err
				}
				return ToolResult{
					Message: fmt.Sprintf("%d genre(s) in the library: %s", len(genres), strings.Join(genres, ", ")),
				}, nil
			},
		},
		{
			Name: "get_queue",
			Description: "List the tracks currently in the play queue, in order, along with which one " +
				"(if any) is currently playing. Check this before replacing or adding to the queue so you " +
				"don't clobber something the user is already listening to without reason, or to answer " +
				"questions about what's playing/queued.",
			InputSchema: map[string]any{},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				tracks := deps.Engine.Queue()
				status := deps.Engine.Status()

				msg := fmt.Sprintf("%d track(s) in the queue.", len(tracks))
				if status.State != "stopped" && status.QueueIndex >= 0 && status.QueueIndex < len(tracks) {
					now := tracks[status.QueueIndex]
					msg += fmt.Sprintf(" Currently %s: #%d %q by %s.", status.State, status.QueueIndex+1, now.Title, now.Artist)
				}

				return ToolResult{
					Message: msg,
					Tracks:  summarize(tracks),
				}, nil
			},
		},
		{
			Name:        "get_favorites",
			Description: "List the user's favorited tracks. Useful for personalizing a request (e.g. weighting selection toward what they already like) or for requests like \"play my favorites\".",
			InputSchema: map[string]any{},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				tracks, err := deps.Lib.ListFavorites()
				if err != nil {
					return ToolResult{}, err
				}
				return ToolResult{
					Message: fmt.Sprintf("%d favorited track(s).", len(tracks)),
					Tracks:  summarize(tracks),
				}, nil
			},
		},
		{
			Name: "set_queue",
			Description: "Replace the current play queue with these tracks, in this order, and start " +
				"playing. Use this for \"play me ...\" requests. track_ids must come from search_library " +
				"or get_favorites results.",
			InputSchema: map[string]any{
				"track_ids": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Track IDs, in play order.",
				},
			},
			Required: []string{"track_ids"},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				var args struct {
					TrackIDs []string `json:"track_ids"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return ToolResult{}, err
				}
				if len(args.TrackIDs) == 0 {
					return ToolResult{}, fmt.Errorf("track_ids is empty")
				}
				if err := deps.Engine.ClearQueue(); err != nil {
					return ToolResult{}, err
				}
				for _, id := range args.TrackIDs {
					if _, err := deps.Engine.Enqueue(id); err != nil {
						return ToolResult{}, fmt.Errorf("queuing track %s: %w", id, err)
					}
				}
				return ToolResult{
					Message:  fmt.Sprintf("Queued %d track(s) and started playback.", len(args.TrackIDs)),
					TrackIDs: args.TrackIDs,
				}, nil
			},
		},
		{
			Name: "create_playlist",
			Description: "Save a new playlist under the given name with these tracks. Fails if a " +
				"playlist with that name already exists — pick a different name or use add_to_playlist. " +
				"track_ids must come from search_library or get_favorites results.",
			InputSchema: map[string]any{
				"name":      map[string]any{"type": "string", "description": "New playlist name."},
				"track_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Track IDs, in order."},
			},
			Required: []string{"name", "track_ids"},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				var args struct {
					Name     string   `json:"name"`
					TrackIDs []string `json:"track_ids"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return ToolResult{}, err
				}
				if args.Name == "" {
					return ToolResult{}, fmt.Errorf("name is required")
				}
				if len(args.TrackIDs) == 0 {
					return ToolResult{}, fmt.Errorf("track_ids is empty")
				}
				_, found, err := deps.Lib.GetPlaylist(args.Name)
				if err != nil {
					return ToolResult{}, err
				}
				if found {
					return ToolResult{}, fmt.Errorf("a playlist named %q already exists — choose another name or use add_to_playlist", args.Name)
				}
				if err := deps.Lib.SavePlaylist(args.Name, args.TrackIDs); err != nil {
					return ToolResult{}, err
				}
				return ToolResult{
					Message:      fmt.Sprintf("Created playlist %q with %d track(s).", args.Name, len(args.TrackIDs)),
					PlaylistName: args.Name,
					TrackIDs:     args.TrackIDs,
				}, nil
			},
		},
		{
			Name:        "add_to_playlist",
			Description: "Add tracks to the end of an existing playlist. Fails if the playlist doesn't exist.",
			InputSchema: map[string]any{
				"name":      map[string]any{"type": "string", "description": "Existing playlist name."},
				"track_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Track IDs to append, in order."},
			},
			Required: []string{"name", "track_ids"},
			Run: func(ctx context.Context, deps *Deps, input json.RawMessage) (ToolResult, error) {
				var args struct {
					Name     string   `json:"name"`
					TrackIDs []string `json:"track_ids"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return ToolResult{}, err
				}
				if _, found, err := deps.Lib.GetPlaylist(args.Name); err != nil {
					return ToolResult{}, err
				} else if !found {
					return ToolResult{}, fmt.Errorf("playlist %q not found", args.Name)
				}
				for _, id := range args.TrackIDs {
					if err := deps.Lib.AddToPlaylist(args.Name, id); err != nil {
						return ToolResult{}, err
					}
				}
				return ToolResult{
					Message:      fmt.Sprintf("Added %d track(s) to %q.", len(args.TrackIDs), args.Name),
					PlaylistName: args.Name,
					TrackIDs:     args.TrackIDs,
				}, nil
			},
		},
	}
}

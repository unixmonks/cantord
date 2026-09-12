export interface Track {
  id: string;
  path: string;
  title: string;
  artist: string;
  album: string;
  album_artist: string;
  album_id: string;
  art_hash?: string;
  track_no: number;
  disc_no: number;
  year: number;
  genre: string;
  duration_ms: number;
  codec: string;
  sample_rate: number;
  bit_depth: number;
  channels: number;
  size: number;
  mtime: number;
  added_at: number;
  favorite: boolean;
  rating: number;
}

export interface Album {
  id: string;
  name: string;
  album_artist: string;
  year: number;
  art_hash?: string;
  track_count: number;
}

export interface Page {
  albums: Album[];
  next_cursor?: string;
}

export interface SearchResult {
  artists: string[];
  albums: Album[];
  tracks: Track[];
}

export type PlaybackState = "stopped" | "playing" | "paused";
export type RepeatMode = "off" | "one" | "all";

export interface Status {
  state: PlaybackState;
  track?: Track;
  queue_index: number;
  position_ms: number;
  duration_ms: number;
  volume: number;
  muted: boolean;
  shuffle: boolean;
  repeat: RepeatMode;
}

export interface ScanProgress {
  running: boolean;
  total: number;
  processed: number;
  added_or_updated: number;
  skipped_unchanged: number;
  marked_missing: number;
  marked_available: number;
  unavailable: number;
  failed: number;
  current_path?: string;
}

export interface Stats {
  tracks: number;
  unavailable: number;
  albums: number;
  artists: number;
  total_size_bytes: number;
  total_duration_ms: number;
}

export interface Playlist {
  name: string;
  track_ids: string[];
}

export interface ApiErrorBody {
  error: string;
}

export interface AiStatus {
  configured: boolean;
  provider: string;
  model: string;
}

export interface AiSuggestion {
  label: string;
  prompt: string;
}

export interface AiConversation {
  id: string;
  title: string;
  created_at: number;
  updated_at: number;
}

export interface AiMessage {
  role: string;
  content: string;
  created_at: number;
}

export interface AiToolCall {
  name: string;
  input: unknown;
}

export interface AiToolResult {
  message: string;
  tracks?: Track[];
  track_ids?: string[];
  playlist_name?: string;
}

// One item in the /api/ai/chat SSE stream. `data`'s shape depends on `type`:
// conversation -> {conversation_id}, text_delta -> string, tool_call ->
// AiToolCall, tool_result -> {name, result: AiToolResult, is_error}, done ->
// {text}, error -> {error}.
export interface AiStreamEvent {
  type: "conversation" | "text_delta" | "tool_call" | "tool_result" | "done" | "error";
  data: unknown;
}

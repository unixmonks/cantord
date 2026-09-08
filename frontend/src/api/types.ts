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
  removed: number;
  failed: number;
  current_path?: string;
}

export interface Stats {
  tracks: number;
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

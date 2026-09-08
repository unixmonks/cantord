import type {
  Album,
  Page,
  Playlist,
  RepeatMode,
  ScanProgress,
  SearchResult,
  Stats,
  Status,
  Track,
} from "./types";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

// Go encodes a nil slice as `null`, not `[]` — every endpoint that can
// return an empty list needs this, or an empty result crashes callers
// that immediately do `.length`/`.map` on the response.
function orEmpty<T>(value: T[] | null | undefined): T[] {
  return value ?? [];
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${baseUrl}${path}`, {
      headers: init?.body ? { "Content-Type": "application/json" } : undefined,
      ...init,
    });
  } catch {
    throw new ApiError(0, `Could not reach cantord at ${baseUrl}`);
  }
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // ignore non-JSON error bodies
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204 || res.status === 202) return undefined as T;
  const text = await res.text();
  return text ? (JSON.parse(text) as T) : (undefined as T);
}

export class ApiClient {
  baseUrl: string;
  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  artUrl(hash: string, size: "thumb" | "full" = "thumb"): string {
    return `${this.baseUrl}/art/${hash}?size=${size}`;
  }

  eventsUrl(): string {
    return `${this.baseUrl}/api/events`;
  }

  // Library / browsing
  async search(q: string, limit?: number) {
    const params = new URLSearchParams({ q });
    if (limit) params.set("limit", String(limit));
    const result = await request<SearchResult>(this.baseUrl, `/api/search?${params}`);
    return { albums: orEmpty(result.albums), tracks: orEmpty(result.tracks) };
  }
  listArtists() {
    return request<string[]>(this.baseUrl, "/api/artists").then(orEmpty);
  }
  async listAlbums(cursor?: string, limit?: number) {
    const params = new URLSearchParams();
    if (cursor) params.set("cursor", cursor);
    if (limit) params.set("limit", String(limit));
    const qs = params.toString();
    const page = await request<Page>(this.baseUrl, `/api/albums${qs ? `?${qs}` : ""}`);
    return { ...page, albums: orEmpty(page.albums) };
  }
  albumIndex() {
    return request<Record<string, string>>(this.baseUrl, "/api/albums/index").then((r) => r ?? {});
  }
  getAlbum(id: string) {
    return request<Album>(this.baseUrl, `/api/albums/${id}`);
  }
  listAlbumTracks(id: string) {
    return request<Track[]>(this.baseUrl, `/api/albums/${id}/tracks`).then(orEmpty);
  }
  getTrack(id: string) {
    return request<Track>(this.baseUrl, `/api/tracks/${id}`);
  }
  recentlyAdded(limit = 50) {
    return request<Track[]>(this.baseUrl, `/api/tracks/recent?limit=${limit}`).then(orEmpty);
  }
  recentlyPlayed(limit = 50) {
    return request<Track[]>(this.baseUrl, `/api/tracks/recently-played?limit=${limit}`).then(orEmpty);
  }
  listFavorites() {
    return request<Track[]>(this.baseUrl, "/api/tracks/favorites").then(orEmpty);
  }
  setFavorite(id: string, favorite: boolean) {
    return request<void>(this.baseUrl, `/api/tracks/${id}/favorite`, {
      method: "POST",
      body: JSON.stringify({ favorite }),
    });
  }
  setRating(id: string, rating: number) {
    return request<void>(this.baseUrl, `/api/tracks/${id}/rating`, {
      method: "POST",
      body: JSON.stringify({ rating }),
    });
  }
  listGenres() {
    return request<string[]>(this.baseUrl, "/api/genres").then(orEmpty);
  }
  tracksByGenre(genre: string) {
    return request<Track[]>(this.baseUrl, `/api/genres/${encodeURIComponent(genre)}/tracks`).then(orEmpty);
  }

  // Queue
  getQueue() {
    return request<Track[]>(this.baseUrl, "/api/queue").then(orEmpty);
  }
  enqueue(trackId: string) {
    return request<Track>(this.baseUrl, "/api/queue", {
      method: "POST",
      body: JSON.stringify({ track_id: trackId }),
    });
  }
  playNext(trackId: string) {
    return request<Track>(this.baseUrl, "/api/queue/next", {
      method: "POST",
      body: JSON.stringify({ track_id: trackId }),
    });
  }
  moveQueue(from: number, to: number) {
    return request<void>(this.baseUrl, "/api/queue/move", {
      method: "POST",
      body: JSON.stringify({ from, to }),
    });
  }
  clearQueue() {
    return request<void>(this.baseUrl, "/api/queue/clear", { method: "POST" });
  }
  removeFromQueue(index: number) {
    return request<void>(this.baseUrl, `/api/queue/${index}`, { method: "DELETE" });
  }
  playIndex(index: number) {
    return request<void>(this.baseUrl, `/api/queue/${index}/play`, { method: "POST" });
  }

  // Playback
  getStatus() {
    return request<Status>(this.baseUrl, "/api/status");
  }
  play() {
    return request<void>(this.baseUrl, "/api/playback/play", { method: "POST" });
  }
  pause() {
    return request<void>(this.baseUrl, "/api/playback/pause", { method: "POST" });
  }
  stop() {
    return request<void>(this.baseUrl, "/api/playback/stop", { method: "POST" });
  }
  next() {
    return request<void>(this.baseUrl, "/api/playback/next", { method: "POST" });
  }
  previous() {
    return request<void>(this.baseUrl, "/api/playback/previous", { method: "POST" });
  }
  seek(positionSeconds: number) {
    return request<void>(this.baseUrl, "/api/playback/seek", {
      method: "POST",
      body: JSON.stringify({ position_seconds: positionSeconds }),
    });
  }
  setVolume(volume: number) {
    return request<void>(this.baseUrl, "/api/playback/volume", {
      method: "POST",
      body: JSON.stringify({ volume }),
    });
  }
  setMute(muted: boolean) {
    return request<void>(this.baseUrl, "/api/playback/mute", {
      method: "POST",
      body: JSON.stringify({ muted }),
    });
  }
  setShuffle(shuffle: boolean) {
    return request<void>(this.baseUrl, "/api/playback/shuffle", {
      method: "POST",
      body: JSON.stringify({ shuffle }),
    });
  }
  setRepeat(mode: RepeatMode) {
    return request<void>(this.baseUrl, "/api/playback/repeat", {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
  }

  // Playlists
  listPlaylists() {
    return request<string[]>(this.baseUrl, "/api/playlists").then(orEmpty);
  }
  savePlaylist(name: string, trackIds: string[]) {
    return request<void>(this.baseUrl, "/api/playlists", {
      method: "POST",
      body: JSON.stringify({ name, track_ids: trackIds }),
    });
  }
  async getPlaylist(name: string) {
    const playlist = await request<Playlist>(this.baseUrl, `/api/playlists/${encodeURIComponent(name)}`);
    return { ...playlist, track_ids: orEmpty(playlist.track_ids) };
  }
  deletePlaylist(name: string) {
    return request<void>(this.baseUrl, `/api/playlists/${encodeURIComponent(name)}`, {
      method: "DELETE",
    });
  }
  addToPlaylist(name: string, trackId: string) {
    return request<void>(this.baseUrl, `/api/playlists/${encodeURIComponent(name)}/tracks`, {
      method: "POST",
      body: JSON.stringify({ track_id: trackId }),
    });
  }
  removeFromPlaylist(name: string, trackId: string) {
    return request<void>(
      this.baseUrl,
      `/api/playlists/${encodeURIComponent(name)}/tracks/${trackId}`,
      { method: "DELETE" },
    );
  }
  renamePlaylist(name: string, newName: string) {
    return request<void>(this.baseUrl, `/api/playlists/${encodeURIComponent(name)}/rename`, {
      method: "POST",
      body: JSON.stringify({ new_name: newName }),
    });
  }

  // Library scan / meta
  triggerScan() {
    return request<void>(this.baseUrl, "/api/library/scan", { method: "POST" });
  }
  scanStatus() {
    return request<ScanProgress>(this.baseUrl, "/api/library/scan/status");
  }
  libraryStats() {
    return request<Stats>(this.baseUrl, "/api/library/stats");
  }
  version() {
    return request<{ version: string }>(this.baseUrl, "/api/version");
  }
  health() {
    return request<{ status: string }>(this.baseUrl, "/api/health");
  }
}

# cantord

A music daemon in the spirit of MPD: it indexes a local library, plays it back
(gapless, hi-res-capable), and exposes control over HTTP/JSON — playlists,
queueing, library browsing, and album art — for any frontend to build on.

## Design

- **Playback: mpv as a subprocess, driven over its JSON IPC socket** — not
  libmpv/cgo. mpv already solves gapless playback, format probing, and
  hi-res/exclusive-mode output on both ALSA (Linux) and OSS (FreeBSD); this
  keeps cantord a plain Go binary that cross-compiles to FreeBSD with no cgo
  toolchain required (verified: `GOOS=freebsd GOARCH=amd64 go build` works
  out of the box).
- **Library index: SQLite** (`modernc.org/sqlite`, pure Go, no cgo) — tracks
  and albums, with an alphabetical `sort_key` for keyset-paginated,
  jump-to-letter browsing. `ffprobe` supplies real stream facts (sample
  rate, bit depth, channels, codec) so hi-res files are actually reported
  as hi-res, not guessed from tags.
- **Album art: content-addressed cache** on disk, keyed by SHA-256 of the
  image bytes. Embedded cover art (via `dhowden/tag`) is extracted at scan
  time; a pluggable `art.Provider` interface (see `internal/art/`) lets
  external sources fill in what's missing — MusicBrainz/Cover Art Archive
  ships as the built-in, no-API-key-required default. A background
  enrichment sweep (`internal/enrich/`) fetches art for albums missing it
  without blocking scans or playback. A third-party provider is expected to
  eventually run out-of-process (e.g. behind `hashicorp/go-plugin`/gRPC)
  behind the same `Provider` interface; that boundary is in place, the
  out-of-process loader isn't built yet.
- **Scan progress:** the HTTP server comes up *before* the initial library
  scan runs, so `GET /api/library/scan/status` and the `scan_progress` SSE
  event are live from daemon start — a large library's first scan isn't a
  silent black box. A fast stat-only pre-pass gives a `total` count so
  progress can be reported as N of M, not just a running counter.
- **Filesystem watching:** `fsnotify` (inotify on Linux, kqueue on FreeBSD —
  one code path for both) triggers a debounced incremental rescan on
  change. Rescans skip any file whose size/mtime hasn't changed.
- **API:** plain `net/http` with Go 1.22+ pattern routing (no router
  dependency). Album art is served over HTTP (`/art/{hash}`), not embedded
  in JSON responses, so it gets normal browser/CDN-style caching
  (`Cache-Control: immutable`) for free. State changes (playback status,
  queue, art becoming available) push over Server-Sent Events at
  `/api/events` so clients don't have to poll.

## Build

```sh
go build ./cmd/cantord
```

Cross-compiles cleanly to FreeBSD:

```sh
GOOS=freebsd GOARCH=amd64 go build ./cmd/cantord
```

Requires `mpv` and `ffprobe` (part of ffmpeg) on `$PATH` at runtime.

## Run

```sh
cp configs/cantord.example.toml ~/.config/cantord/cantord.toml
# edit music_dirs
./cantord -config ~/.config/cantord/cantord.toml
```

With no `-config`, cantord still runs with built-in defaults (`~/.local/share/cantord/...`)
but `library.music_dirs` has no sane default and must be set via a config
file.

## API

All endpoints are JSON in/out except `/art/{hash}`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/albums?cursor=&limit=` | Alphabetical, keyset-paginated album list |
| GET | `/api/albums/index` | `{"A": cursor, "B": cursor, ...}` for jump-to-letter |
| GET | `/api/albums/{id}` | Album detail |
| GET | `/api/albums/{id}/tracks` | Tracks in an album |
| GET | `/art/{hash}?size=thumb\|full` | Album art (immutable-cacheable) |
| GET | `/api/queue` | Current play queue |
| POST | `/api/queue` `{track_id}` | Enqueue (plays immediately if queue was empty) |
| POST | `/api/queue/clear` | Clear queue |
| DELETE | `/api/queue/{index}` | Remove one queue entry |
| POST | `/api/queue/{index}/play` | Jump playback to a queue entry |
| GET | `/api/status` | Current playback status |
| POST | `/api/playback/{play,pause,stop,next,previous}` | Transport control |
| POST | `/api/playback/seek` `{position_seconds}` | Seek |
| POST | `/api/playback/volume` `{volume}` | 0-100 |
| GET/POST | `/api/playlists` | List / save a named playlist |
| GET/DELETE | `/api/playlists/{name}` | Load / delete a named playlist |
| POST | `/api/library/scan` | Trigger a rescan (async; watch `/api/events` or poll status below) |
| GET | `/api/library/scan/status` | Poll-friendly scan progress: `{running, total, processed, added_or_updated, skipped_unchanged, failed, current_path}` |
| GET | `/api/events` | SSE: `status`, `queue_changed`, `library_changed`, `scan_progress`, `art_updated` |

## What's implemented vs. not

Implemented and tested end-to-end (real hi-res FLAC + embedded-art MP3
fixtures, full scan → queue → play → pause → volume → playlist CRUD →
SSE → graceful shutdown cycle):

- Incremental, fingerprinted library scanning + fs-watch rescans
- Real stream introspection via ffprobe (sample rate/bit depth/channels/codec)
- Content-addressed art cache with thumbnail generation and embedded-art extraction
- mpv-backed gapless playback with a mirrored queue
- Full HTTP API above, including SSE push

Not built yet (documented as the obvious next steps, not silently missing):

- Out-of-process/third-party art & metadata plugins (the `Provider`
  interface is the intended seam — see `internal/art/provider.go`)
- Saved-playlist ordering edits (only whole-playlist save/replace exists)
- Multi-user auth/permissions — currently a single trusted local API
- Output device enumeration/selection API (device is config-only for now)

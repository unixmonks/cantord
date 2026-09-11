# cantord

A music daemon in the spirit of MPD, plus a web UI and a terminal UI.
Three clients, one repo:

- **[`backend/`](backend)** — `cantord`, the Go daemon: library indexing,
  mpv-backed gapless playback, and an HTTP/JSON + SSE API. See
  [`backend/README.md`](backend/README.md) for design notes, build/run
  instructions, and the full API reference.
- **[`frontend/`](frontend)** — `cantord-frontend`, a React/TypeScript web
  UI that talks to the daemon's API. See
  [`frontend/README.md`](frontend/README.md) for dev setup.
- **[`backend/cmd/cantord-tui`](backend/cmd/cantord-tui)** — `cantord-tui`,
  a full-screen, keyboard-driven terminal client for the same API. Build
  with `go build ./cmd/cantord-tui` from `backend/` and run the resulting
  binary against a running daemon.

They're decoupled at runtime — the frontend is a plain browser client
pointed at a daemon address (configurable in Settings, CORS-open by
design) — so each can be built, deployed, and versioned independently.

## Screenshots

<table>
<tr>
<td width="50%">

**Home**
![Home page](docs/screenshots/home.jpg)

</td>
<td width="50%">

**Queue**
![Queue page](docs/screenshots/queue.jpg)

</td>
</tr>
<tr>
<td width="50%">

**Albums**
![Albums grid](docs/screenshots/albums.jpg)

</td>
<td width="50%">

**Album detail**
![Album detail page](docs/screenshots/album-detail.jpg)

</td>
</tr>
</table>

**Player bar** — persistent across every page, with transport, seek, and
the animated now-playing indicator:

![Player bar](docs/screenshots/player-bar.png)

**AI assistant** — chat-driven queue/playlist building, live against a real
library and a real Claude API key (see [Features](#features) below):

<img src="docs/screenshots/ai-assistant.png" width="360" alt="AI assistant panel queuing 90s grunge tracks">

### `cantord-tui`

Miller columns for drilling into artists/genres/playlists, a global fuzzy
search overlay, and the queue view:

<table>
<tr>
<td width="33%">

**Artists → Album → Tracks**
![Miller columns navigation](docs/screenshots/tui/miller-columns.png)

</td>
<td width="33%">

**Fuzzy search**
![Fuzzy search overlay](docs/screenshots/tui/search.png)

</td>
<td width="33%">

**Genres**
![Genre drill-down](docs/screenshots/tui/genres.png)

</td>
</tr>
<tr>
<td width="33%">

**Queue**
![Queue view](docs/screenshots/tui/queue.png)

</td>
<td width="33%">

**Playlists**
![Playlists view](docs/screenshots/tui/playlists.png)

</td>
<td width="33%"></td>
</tr>
</table>

## Features

### Backend (`cantord`)

- Incremental, fingerprinted library scanning with filesystem watching
  (inotify/kqueue) for automatic rescans on change
- mpv-backed gapless playback; the queue and playback position are
  checkpointed to the database so a restart or crash resumes where you
  left off
- Real stream introspection via `ffprobe` (sample rate, bit depth,
  channels, codec) — hi-res files are reported as hi-res, not guessed
- Content-addressed album art cache with thumbnail generation,
  embedded-art extraction, and MusicBrainz/Cover Art Archive enrichment
  for what's missing
- Saved playlists (create, rename, delete, per-track add/remove/reorder),
  favorites, and ratings
- Full HTTP/JSON API with Server-Sent Events for live status, queue, and
  library-change push — no polling
- **AI assistant**: an optional LLM tool-calling layer (Claude API) that
  can search the library and build a queue or playlist from a plain
  chat request — it only ever acts on tracks a search actually
  returned, never an invented one. Runs fine with no API key configured
- Pure Go, no cgo — cross-compiles cleanly to FreeBSD

### Frontend (`cantord-frontend`)

- Home, Albums (jump-to-letter index, infinite scroll), Artists, Genres,
  Search, Playlists, Favorites, Recently Added/Played, and Queue views
- A slide-out **AI assistant** panel, streamed live, with tool activity
  shown inline and confirmation cards linking to the queue/playlist it
  built
- Drag-to-reorder playlists, and an "add to playlist" menu (existing or
  new) available from any track row or album
- Full playback transport (play/pause/seek/volume/shuffle/repeat) in a
  persistent player bar, with an animated now-playing indicator
- Real-time UI via the backend's SSE stream — queue, playback status, and
  library changes reflect instantly across the app, with a visible
  disconnected state if the daemon goes away
- Collapsible sidebar with per-section navigation memory (returning to a
  section resumes wherever you left it, not its list root)

### Terminal (`cantord-tui`)

- Tab bar (Artists, Albums, Genres, Queue, Playlists) with Miller-column
  drill-down in each section — browse artist → album → tracks without
  leaving the keyboard
- Global fuzzy search overlay across artists, albums, and tracks, jumping
  straight to a result's place in the library
- Vim-style navigation throughout (`j`/`k`, `g`/`G`, `/` to filter, `dd`
  to remove from a playlist/queue) plus a full-screen `?` help overlay
  listing every shortcut
- Full playback control (play/pause, seek, volume, shuffle, repeat),
  playlist management, and a persistent footer with now-playing info,
  progress bar, and live daemon connection status over SSE
- Configurable keyboard shortcuts via an optional TOML file
  (`-keys`/`CANTORD_TUI_KEYS`) and a choice of built-in themes (dracula,
  nord, gruvbox, catppuccin) or an adaptive default palette
  (`-theme`/`CANTORD_TUI_THEME`)
- Pure Go, single static binary, no browser required

## Quickstart

```sh
# backend
cd backend
go build ./cmd/cantord
cp configs/cantord.example.toml ~/.config/cantord/cantord.toml   # edit music_dirs
./cantord -config ~/.config/cantord/cantord.toml

# frontend, in another shell
cd frontend
npm install
npm run dev

# terminal UI, instead of (or alongside) the frontend
cd backend
go build ./cmd/cantord-tui
./cantord-tui   # reads CANTORD_ADDR, defaults to http://localhost:8080
```

Open the frontend's dev URL, then set the daemon address in Settings
(defaults to `http://localhost:8080`).

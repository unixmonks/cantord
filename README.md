# cantord

A music daemon in the spirit of MPD, plus a web UI. Two independent
projects in one repo:

- **[`backend/`](backend)** — `cantord`, the Go daemon: library indexing,
  mpv-backed gapless playback, and an HTTP/JSON + SSE API. See
  [`backend/README.md`](backend/README.md) for design notes, build/run
  instructions, and the full API reference.
- **[`frontend/`](frontend)** — `cantord-frontend`, a React/TypeScript web
  UI that talks to the daemon's API. See
  [`frontend/README.md`](frontend/README.md) for dev setup.

They're decoupled at runtime — the frontend is a plain browser client
pointed at a daemon address (configurable in Settings, CORS-open by
design) — so each can be built, deployed, and versioned independently.

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
```

Open the frontend's dev URL, then set the daemon address in Settings
(defaults to `http://localhost:8080`).

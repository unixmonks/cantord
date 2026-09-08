# cantord-frontend

React + TypeScript web UI for [cantord](../backend). Talks to the daemon's
HTTP/JSON API over plain `fetch`/SSE — no build-time coupling to the backend,
so it can run against any cantord instance reachable from the browser.

## Develop

```sh
npm install
npm run dev
```

The dev server binds `0.0.0.0` (see `vite.config.ts`) so it's reachable from
other devices on the network, e.g. when testing from a phone.

On first load, set the daemon's address in Settings (defaults to
`http://localhost:8080`, stored in `localStorage`). The backend's CORS
handling (see `backend/internal/api/server.go`) allows the browser to reach
it from any origin, so no dev proxy is needed.

## Build

```sh
npm run build   # tsc -b && vite build, output in dist/
npm run preview # serve the production build locally
```

## Lint

```sh
npm run lint
```

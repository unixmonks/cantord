// Package enrich runs the background art-enrichment pipeline: for albums
// with no embedded/local art, ask the art.Registry's external providers,
// cache what comes back, and publish an event so connected clients can swap
// the image in without re-fetching the album list.
package enrich

import (
	"context"
	"log/slog"
	"time"

	"cantord/internal/art"
	"cantord/internal/events"
	"cantord/internal/library"
)

type Enricher struct {
	lib      *library.Store
	artStore *art.Store
	registry *art.Registry
	bus      *events.Bus
}

func New(lib *library.Store, artStore *art.Store, registry *art.Registry, bus *events.Bus) *Enricher {
	return &Enricher{lib: lib, artStore: artStore, registry: registry, bus: bus}
}

// RunOnce sweeps every album currently missing art and tries to fill it in.
// Safe to call after every scan; already-covered albums are skipped cheaply.
func (e *Enricher) RunOnce(ctx context.Context) {
	albums, err := e.lib.ListAlbumsMissingArt()
	if err != nil {
		slog.Error("enrich: listing albums missing art", "err", err)
		return
	}
	if len(albums) == 0 {
		return
	}
	slog.Info("enrich: fetching art for albums", "count", len(albums))

	for _, album := range albums {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, ok := e.registry.ResolveAlbumArt(ctx, art.FetchRequest{
			Artist: album.AlbumArtist,
			Album:  album.Name,
		})
		if !ok {
			continue
		}
		hash, err := e.artStore.Put(result.Data)
		if err != nil {
			slog.Warn("enrich: caching art", "album", album.Name, "err", err)
			continue
		}
		if err := e.lib.SetAlbumArt(album.ID, hash); err != nil {
			slog.Warn("enrich: saving art hash", "album", album.Name, "err", err)
			continue
		}
		e.bus.Publish(events.Event{Type: "art_updated", Data: map[string]string{
			"album_id": album.ID,
			"art_hash": hash,
			"source":   result.Source,
		}})
	}
}

// RunPeriodically calls RunOnce on an interval until ctx is cancelled — a
// simple sweep is enough at library scale; no need for a persistent job
// queue for an MVP.
func (e *Enricher) RunPeriodically(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.RunOnce(ctx)
		}
	}
}

// Command cantord is a music daemon: it indexes a local library, drives mpv
// for gapless/hi-res playback, and exposes queue/playlist/library control
// plus album art over HTTP.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cantord/internal/api"
	"cantord/internal/art"
	"cantord/internal/config"
	"cantord/internal/enrich"
	"cantord/internal/events"
	"cantord/internal/library"
	"cantord/internal/playback"
)

func main() {
	configPath := flag.String("config", "", "path to cantord.toml (optional; built-in defaults are used otherwise)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lib, err := library.Open(cfg.Library.DBPath)
	if err != nil {
		slog.Error("opening library store", "err", err)
		os.Exit(1)
	}
	defer lib.Close()

	artStore, err := art.New(cfg.Library.ArtCacheDir)
	if err != nil {
		slog.Error("opening art store", "err", err)
		os.Exit(1)
	}

	bus := events.NewBus()
	scanner := library.NewScanner(lib, artStore, bus, ffprobePath(), cfg.Library.MusicDirs)

	registry := art.NewRegistry(0.5)
	registry.Register(art.NewMusicBrainzProvider("cantord/0.1 (+https://github.com/unixmonks/cantord)"))
	enricher := enrich.New(lib, artStore, registry, bus)

	mpvClient, err := playback.Start(cfg.Playback.MPVPath, cfg.Playback.IPCSocket, cfg.Playback.AO, cfg.Playback.AudioDevice)
	if err != nil {
		slog.Error("starting mpv", "err", err)
		os.Exit(1)
	}
	engine := playback.NewEngine(mpvClient, lib, bus)
	go engine.Run(ctx)

	server := api.New(lib, artStore, engine, bus, scanner)
	httpServer := &http.Server{Addr: cfg.Server.Listen, Handler: server.Router()}

	go func() {
		slog.Info("cantord listening", "addr", cfg.Server.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "err", err)
		}
	}()

	// Run the initial scan after the HTTP server is already listening, so
	// GET /api/library/scan/status and the scan_progress SSE event are
	// live from the moment the daemon starts — a large library's first
	// scan can take a while and shouldn't be a black box.
	go func() {
		if err := scanner.Scan(ctx); err != nil {
			slog.Error("initial library scan", "err", err)
			return
		}
		enricher.RunOnce(ctx)
		if err := scanner.Watch(ctx, 2*time.Second); err != nil {
			slog.Warn("library watch stopped", "err", err)
		}
	}()
	go enricher.RunPeriodically(ctx, 10*time.Minute)

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	engine.Shutdown()
}

func ffprobePath() string {
	if p := os.Getenv("CANTORD_FFPROBE"); p != "" {
		return p
	}
	return "ffprobe"
}

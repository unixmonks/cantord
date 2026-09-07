package playback

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cantord/internal/events"
	"cantord/internal/library"
)

type Status struct {
	State      string         `json:"state"` // "stopped" | "playing" | "paused"
	Track      *library.Track `json:"track,omitempty"`
	QueueIndex int            `json:"queue_index"`
	PositionMS int            `json:"position_ms"`
	DurationMS int            `json:"duration_ms"`
	Volume     float64        `json:"volume"`
}

// Engine owns the play queue and mirrors it 1:1 onto mpv's internal
// playlist (same order, same indices) so mpv can be trusted for gapless
// transitions while cantord stays the source of truth for track metadata.
type Engine struct {
	mpv *Client
	lib *library.Store
	bus *events.Bus

	mu     sync.Mutex
	queue  []library.Track
	paused bool
	pos    int // playlist-pos as last reported by mpv; -1 = no current track
	posMS  int
	durMS  int
	volume float64
}

func NewEngine(mpv *Client, lib *library.Store, bus *events.Bus) *Engine {
	return &Engine{mpv: mpv, lib: lib, bus: bus, pos: -1, volume: 100}
}

// Run consumes mpv's IPC events and keeps Status current until ctx is
// cancelled. Call it in its own goroutine.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-e.mpv.Events():
			if !ok {
				return
			}
			e.handleEvent(ev)
		case <-ticker.C:
			e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
		}
	}
}

func (e *Engine) handleEvent(ev RawEvent) {
	if ev["event"] != "property-change" {
		return
	}
	name, _ := ev["name"].(string)

	e.mu.Lock()
	switch name {
	case "pause":
		e.paused, _ = ev["data"].(bool)
	case "volume":
		e.volume, _ = ev["data"].(float64)
	case "time-pos":
		if v, ok := ev["data"].(float64); ok {
			e.posMS = int(v * 1000)
		}
	case "duration":
		if v, ok := ev["data"].(float64); ok {
			e.durMS = int(v * 1000)
		}
	case "playlist-pos":
		if v, ok := ev["data"].(float64); ok {
			e.pos = int(v)
		} else {
			e.pos = -1
		}
	}
	e.mu.Unlock()

	if name == "pause" || name == "playlist-pos" {
		e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
	}
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()

	state := "stopped"
	var track *library.Track
	if e.pos >= 0 && e.pos < len(e.queue) {
		t := e.queue[e.pos]
		track = &t
		if e.paused {
			state = "paused"
		} else {
			state = "playing"
		}
	}
	return Status{
		State:      state,
		Track:      track,
		QueueIndex: e.pos,
		PositionMS: e.posMS,
		DurationMS: e.durMS,
		Volume:     e.volume,
	}
}

func (e *Engine) Queue() []library.Track {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]library.Track, len(e.queue))
	copy(out, e.queue)
	return out
}

// Enqueue appends a track to both cantord's queue and mpv's playlist. If
// the queue was empty, playback starts immediately (append-play); otherwise
// mpv gapless-transitions into it in order once earlier tracks finish.
func (e *Engine) Enqueue(trackID string) (library.Track, error) {
	track, found, err := e.lib.GetTrack(trackID)
	if err != nil {
		return library.Track{}, err
	}
	if !found {
		return library.Track{}, fmt.Errorf("track %s not found", trackID)
	}

	e.mu.Lock()
	mode := "append"
	if len(e.queue) == 0 {
		mode = "append-play"
	}
	e.queue = append(e.queue, track)
	e.mu.Unlock()

	if err := e.mpv.LoadFile(track.Path, mode); err != nil {
		return library.Track{}, err
	}
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	return track, nil
}

func (e *Engine) RemoveFromQueue(index int) error {
	e.mu.Lock()
	if index < 0 || index >= len(e.queue) {
		e.mu.Unlock()
		return fmt.Errorf("index %d out of range", index)
	}
	e.queue = append(e.queue[:index], e.queue[index+1:]...)
	e.mu.Unlock()

	if err := e.mpv.PlaylistRemove(index); err != nil {
		return err
	}
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	return nil
}

func (e *Engine) ClearQueue() error {
	e.mu.Lock()
	e.queue = nil
	e.mu.Unlock()

	if err := e.mpv.PlaylistClear(); err != nil {
		return err
	}
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	return nil
}

func (e *Engine) PlayIndex(index int) error {
	e.mu.Lock()
	inRange := index >= 0 && index < len(e.queue)
	e.mu.Unlock()
	if !inRange {
		return fmt.Errorf("index %d out of range", index)
	}
	return e.mpv.PlaylistPlayIndex(index)
}

func (e *Engine) Play() error     { return e.mpv.SetPause(false) }
func (e *Engine) Pause() error    { return e.mpv.SetPause(true) }
func (e *Engine) Stop() error     { return e.mpv.Stop() }
func (e *Engine) Next() error     { return e.mpv.PlaylistNext() }
func (e *Engine) Previous() error { return e.mpv.PlaylistPrev() }

func (e *Engine) Seek(seconds float64) error { return e.mpv.Seek(seconds) }

func (e *Engine) SetVolume(v float64) error {
	if v < 0 || v > 100 {
		return fmt.Errorf("volume must be 0-100")
	}
	return e.mpv.SetVolume(v)
}

// Shutdown stops mpv cleanly.
func (e *Engine) Shutdown() {
	if err := e.mpv.Close(); err != nil {
		slog.Warn("engine: closing mpv", "err", err)
	}
}

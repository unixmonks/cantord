package playback

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
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
	Muted      bool           `json:"muted"`
	Shuffle    bool           `json:"shuffle"`
	Repeat     string         `json:"repeat"` // "off" | "one" | "all"
}

// Engine owns the play queue and mirrors it 1:1 onto mpv's internal
// playlist (same order, same indices) so mpv can be trusted for gapless
// transitions while cantord stays the source of truth for track metadata.
type Engine struct {
	mpv *Client
	lib *library.Store
	bus *events.Bus

	mu      sync.Mutex
	queue   []library.Track
	paused  bool
	pos     int // playlist-pos as last reported by mpv; -1 = no current track
	posMS   int
	durMS   int
	volume  float64
	muted   bool
	shuffle bool
	repeat  string // "off" | "one" | "all"
}

func NewEngine(mpv *Client, lib *library.Store, bus *events.Bus) *Engine {
	return &Engine{mpv: mpv, lib: lib, bus: bus, pos: -1, volume: 100, repeat: "off"}
}

// Run consumes mpv's IPC events and keeps Status current until ctx is
// cancelled. Call it in its own goroutine.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	// Separate, slower ticker so a crash (no chance to run Shutdown) loses at
	// most a few seconds of playback position, without writing to the queue
	// database once per second forever.
	checkpoint := time.NewTicker(5 * time.Second)
	defer checkpoint.Stop()
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
		case <-checkpoint.C:
			e.persistQueue()
		}
	}
}

// persistQueue checkpoints the current queue and playback position so
// Restore can bring it back after a restart or crash. Logged failures are
// non-fatal — the running queue is unaffected, only its next-startup
// recovery.
func (e *Engine) persistQueue() {
	e.mu.Lock()
	ids := make([]string, len(e.queue))
	for i, t := range e.queue {
		ids[i] = t.ID
	}
	pos, posMS := e.pos, e.posMS
	e.mu.Unlock()

	if err := e.lib.SaveQueueState(ids, pos, posMS); err != nil {
		slog.Warn("engine: persisting queue", "err", err)
	}
}

// Restore reloads the queue and playback position saved by the previous
// run so a restart or crash doesn't lose what was queued. Tracks no
// longer in the library (deleted/moved since) are skipped. Restored
// playback always starts paused — resuming with sound blasting the moment
// the daemon comes up would be a bad surprise; the user presses play.
func (e *Engine) Restore() {
	ids, savedPos, savedPosMS, err := e.lib.LoadQueueState()
	if err != nil {
		slog.Warn("engine: loading saved queue", "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	var tracks []library.Track
	pos := -1
	for i, id := range ids {
		track, found, err := e.lib.GetTrack(id)
		if err != nil {
			slog.Warn("engine: resolving saved queue track", "track_id", id, "err", err)
			continue
		}
		if !found {
			continue
		}
		if i == savedPos {
			pos = len(tracks)
		}
		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		return
	}
	if pos < 0 {
		pos = 0
	}

	if err := e.mpv.SetPause(true); err != nil {
		slog.Warn("engine: restoring queue", "err", err)
		return
	}
	for _, t := range tracks {
		if err := e.mpv.LoadFile(t.Path, "append"); err != nil {
			slog.Warn("engine: restoring queue", "track_id", t.ID, "err", err)
			return
		}
	}
	if err := e.mpv.PlaylistPlayIndex(pos); err != nil {
		slog.Warn("engine: restoring queue position", "err", err)
	}
	if savedPosMS > 0 {
		// Switching tracks is asynchronous on mpv's side — the command above
		// acknowledges instantly, but mpv only finishes opening the file (and
		// would otherwise clobber a too-early seek back to 0) a beat later.
		// Give it a moment before seeking.
		time.Sleep(300 * time.Millisecond)
		if err := e.mpv.Seek(float64(savedPosMS) / 1000); err != nil {
			slog.Warn("engine: restoring seek position", "err", err)
		}
	}

	e.mu.Lock()
	e.queue = tracks
	e.pos = pos
	e.posMS = savedPosMS
	e.paused = true
	e.mu.Unlock()

	slog.Info("engine: restored queue", "tracks", len(tracks), "position", pos)
}

func (e *Engine) handleEvent(ev RawEvent) {
	if ev["event"] != "property-change" {
		return
	}
	name, _ := ev["name"].(string)

	e.mu.Lock()
	var newlyPlayingTrackID string
	switch name {
	case "pause":
		e.paused, _ = ev["data"].(bool)
	case "volume":
		e.volume, _ = ev["data"].(float64)
	case "mute":
		e.muted, _ = ev["data"].(bool)
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
			if e.pos >= 0 && e.pos < len(e.queue) {
				newlyPlayingTrackID = e.queue[e.pos].ID
			}
		} else {
			e.pos = -1
		}
	}
	e.mu.Unlock()

	if newlyPlayingTrackID != "" {
		e.mu.Lock()
		track := e.queue[e.pos]
		e.mu.Unlock()
		slog.Info("engine: now playing", "track_id", track.ID, "title", track.Title, "artist", track.Artist)

		if err := e.lib.RecordPlay(newlyPlayingTrackID); err != nil {
			slog.Warn("engine: recording play history", "err", err)
		}
	}

	// Checkpoint immediately on a track change or pause/resume — the
	// periodic checkpoint in Run covers position drift during a single
	// track, but these are the moments most worth not losing on a crash.
	if name == "playlist-pos" || name == "pause" {
		e.persistQueue()
	}

	if name == "pause" || name == "playlist-pos" || name == "mute" {
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
		Muted:      e.muted,
		Shuffle:    e.shuffle,
		Repeat:     e.repeat,
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
	slog.Info("engine: enqueue", "track_id", track.ID, "title", track.Title, "artist", track.Artist, "mode", mode)
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.persistQueue()
	return track, nil
}

// PlayNext inserts a track immediately after the currently-playing entry
// (or at the front if nothing's queued yet), so it's the next thing to
// play without disturbing the rest of the queue.
func (e *Engine) PlayNext(trackID string) (library.Track, error) {
	track, found, err := e.lib.GetTrack(trackID)
	if err != nil {
		return library.Track{}, err
	}
	if !found {
		return library.Track{}, fmt.Errorf("track %s not found", trackID)
	}

	e.mu.Lock()
	insertAt := e.pos + 1
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(e.queue) {
		insertAt = len(e.queue)
	}
	empty := len(e.queue) == 0
	e.queue = append(e.queue, library.Track{})
	copy(e.queue[insertAt+1:], e.queue[insertAt:])
	e.queue[insertAt] = track
	lastIdx := len(e.queue) - 1
	e.mu.Unlock()

	mode := "append"
	if empty {
		mode = "append-play"
	}
	if err := e.mpv.LoadFile(track.Path, mode); err != nil {
		return library.Track{}, err
	}
	if !empty && lastIdx != insertAt {
		if err := e.mpv.PlaylistMove(lastIdx, insertAt); err != nil {
			return library.Track{}, err
		}
	}
	slog.Info("engine: play next", "track_id", track.ID, "title", track.Title, "artist", track.Artist)
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.persistQueue()
	return track, nil
}

// MoveInQueue reorders the queue, moving the entry at "from" so it takes
// the place of the entry at "to" — same semantics as mpv's playlist-move.
func (e *Engine) MoveInQueue(from, to int) error {
	e.mu.Lock()
	if from < 0 || from >= len(e.queue) || to < 0 || to >= len(e.queue) {
		e.mu.Unlock()
		return fmt.Errorf("index out of range")
	}
	e.queue = moveTrack(e.queue, from, to)
	e.mu.Unlock()

	if err := e.mpv.PlaylistMove(from, to); err != nil {
		return err
	}
	slog.Info("engine: move queue", "from", from, "to", to)
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.persistQueue()
	return nil
}

func moveTrack(q []library.Track, from, to int) []library.Track {
	item := q[from]
	rest := make([]library.Track, 0, len(q)-1)
	rest = append(rest, q[:from]...)
	rest = append(rest, q[from+1:]...)
	if to > len(rest) {
		to = len(rest)
	}
	out := make([]library.Track, 0, len(q))
	out = append(out, rest[:to]...)
	out = append(out, item)
	out = append(out, rest[to:]...)
	return out
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
	slog.Info("engine: remove from queue", "index", index)
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.persistQueue()
	return nil
}

func (e *Engine) ClearQueue() error {
	e.mu.Lock()
	e.queue = nil
	e.pos = -1
	e.mu.Unlock()

	// mpv's "playlist-clear" command deliberately keeps the currently-playing
	// entry, which would leave mpv's playlist out of sync with e.queue (now
	// empty) and the old track still audible. "stop" clears the playlist
	// (including the current entry) and halts playback to match.
	if err := e.mpv.Stop(); err != nil {
		return err
	}
	slog.Info("engine: clear queue")
	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
	e.persistQueue()
	return nil
}

func (e *Engine) PlayIndex(index int) error {
	e.mu.Lock()
	inRange := index >= 0 && index < len(e.queue)
	e.mu.Unlock()
	if !inRange {
		return fmt.Errorf("index %d out of range", index)
	}
	if err := e.mpv.PlaylistPlayIndex(index); err != nil {
		return err
	}
	slog.Info("engine: play index", "index", index)
	// Switching tracks doesn't clear mpv's pause flag on its own — without
	// this, picking a track while paused loads it but leaves it paused.
	return e.mpv.SetPause(false)
}

func (e *Engine) Play() error {
	slog.Info("engine: play")
	return e.mpv.SetPause(false)
}

func (e *Engine) Pause() error {
	slog.Info("engine: pause")
	return e.mpv.SetPause(true)
}

func (e *Engine) Stop() error {
	slog.Info("engine: stop")
	return e.mpv.Stop()
}

func (e *Engine) Next() error {
	slog.Info("engine: next")
	return e.mpv.PlaylistNext()
}

func (e *Engine) Previous() error {
	slog.Info("engine: previous")
	return e.mpv.PlaylistPrev()
}

func (e *Engine) Seek(seconds float64) error {
	slog.Info("engine: seek", "seconds", seconds)
	return e.mpv.Seek(seconds)
}

func (e *Engine) SetVolume(v float64) error {
	if v < 0 || v > 100 {
		return fmt.Errorf("volume must be 0-100")
	}
	slog.Info("engine: set volume", "volume", v)
	return e.mpv.SetVolume(v)
}

func (e *Engine) SetMute(muted bool) error {
	if err := e.mpv.SetMute(muted); err != nil {
		return err
	}
	slog.Info("engine: set mute", "muted", muted)
	e.mu.Lock()
	e.muted = muted
	e.mu.Unlock()
	e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
	return nil
}

func (e *Engine) SetRepeat(mode string) error {
	switch mode {
	case "off", "one", "all":
	default:
		return fmt.Errorf("repeat mode must be off, one, or all")
	}

	if err := e.mpv.SetLoopFile(mode == "one"); err != nil {
		return err
	}
	if err := e.mpv.SetLoopPlaylist(mode == "all"); err != nil {
		return err
	}

	slog.Info("engine: set repeat", "mode", mode)
	e.mu.Lock()
	e.repeat = mode
	e.mu.Unlock()
	e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
	return nil
}

// SetShuffle randomizes the not-yet-played tail of the queue when turned
// on. It's a one-shot shuffle of what's left, not a running mode: turning
// shuffle back off does not restore the pre-shuffle order.
func (e *Engine) SetShuffle(on bool) error {
	slog.Info("engine: set shuffle", "shuffle", on)

	e.mu.Lock()
	if e.shuffle == on {
		e.mu.Unlock()
		return nil
	}
	e.shuffle = on

	var tail []library.Track
	if on {
		start := e.pos + 1
		if start < 0 {
			start = 0
		}
		if start < len(e.queue) {
			tail = append([]library.Track{}, e.queue[start:]...)
			rand.Shuffle(len(tail), func(i, j int) { tail[i], tail[j] = tail[j], tail[i] })
			e.queue = append(e.queue[:start:start], tail...)
		}
	}
	e.mu.Unlock()

	if len(tail) > 0 {
		if err := e.mpv.PlaylistClear(); err != nil { // keeps the currently-playing entry
			return err
		}
		for _, t := range tail {
			if err := e.mpv.LoadFile(t.Path, "append"); err != nil {
				return err
			}
		}
	}

	e.bus.Publish(events.Event{Type: "queue_changed", Data: e.Queue()})
	e.bus.Publish(events.Event{Type: "status", Data: e.Status()})
	e.persistQueue()
	return nil
}

// Shutdown stops mpv cleanly, checkpointing the queue one last time first
// so a graceful restart resumes from the freshest possible position.
func (e *Engine) Shutdown() {
	e.persistQueue()
	if err := e.mpv.Close(); err != nil {
		slog.Warn("engine: closing mpv", "err", err)
	}
}

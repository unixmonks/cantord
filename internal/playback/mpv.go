// Package playback drives mpv as a subprocess over its JSON IPC socket —
// deliberately not libmpv/cgo, so cantord stays a single static-ish Go
// binary that cross-compiles cleanly to FreeBSD. mpv itself already handles
// gapless playback, format probing, and hi-res/exclusive-mode output on
// both ALSA (Linux) and OSS (FreeBSD); this package just talks to it.
package playback

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// RawEvent is an mpv IPC event, kept generic since the fields present
// depend on the event type (property-change carries name/data, end-file
// carries reason, etc.) — callers pull out what they need.
type RawEvent map[string]any

type Client struct {
	cmd    *exec.Cmd
	conn   net.Conn
	events chan RawEvent

	mu      sync.Mutex
	pending map[int64]chan json.RawMessage
	nextID  int64
	closed  atomic.Bool
}

type ipcRequest struct {
	Command   []any `json:"command"`
	RequestID int64 `json:"request_id"`
}

type ipcResponse struct {
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
}

// Start launches mpv in idle mode with an IPC socket and connects to it.
// ao, if non-empty, forces mpv's output driver (--ao, e.g. "alsa"/"oss"/
// "null"). audioDevice, if non-empty, picks a specific device within that
// driver (e.g. "alsa/hw:CARD=DAC,DEV=0" for bit-perfect exclusive-mode
// output, bypassing dmix/resampling).
func Start(mpvPath, socketPath, ao, audioDevice string) (*Client, error) {
	_ = os.Remove(socketPath)

	args := []string{
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--no-audio-display",
		"--gapless-audio=yes",
		"--input-ipc-server=" + socketPath,
	}
	if ao != "" {
		args = append(args, "--ao="+ao)
	}
	if audioDevice != "" {
		args = append(args, "--audio-device="+audioDevice)
	}

	cmd := exec.Command(mpvPath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting mpv: %w", err)
	}

	var conn net.Conn
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("connecting to mpv ipc socket: %w", err)
	}

	c := &Client{
		cmd:     cmd,
		conn:    conn,
		events:  make(chan RawEvent, 64),
		pending: make(map[int64]chan json.RawMessage),
	}
	go c.readLoop()

	for _, prop := range []string{"pause", "time-pos", "duration", "volume", "playlist-pos", "media-title"} {
		_, _ = c.Command("observe_property", 0, prop)
	}

	return c, nil
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()

		var resp ipcResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.RequestID != 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.RequestID]
			delete(c.pending, resp.RequestID)
			c.mu.Unlock()
			if ok {
				if resp.Error != "" && resp.Error != "success" {
					ch <- nil
				} else {
					ch <- resp.Data
				}
				close(ch)
			}
			continue
		}

		var ev RawEvent
		if err := json.Unmarshal(line, &ev); err == nil && ev["event"] != nil {
			slog.Debug("mpv: event", "event", ev)
			select {
			case c.events <- ev:
			default: // slow consumer: drop rather than block mpv's IPC
			}
		}
	}
	close(c.events)
}

// Command sends a raw mpv IPC command and waits for its response.
func (c *Client) Command(args ...any) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("mpv client closed")
	}
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan json.RawMessage, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	payload, err := json.Marshal(ipcRequest{Command: args, RequestID: id})
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')

	slog.Debug("mpv: command", "args", args)
	if _, err := c.conn.Write(payload); err != nil {
		return nil, fmt.Errorf("writing mpv command: %w", err)
	}

	select {
	case data := <-ch:
		slog.Debug("mpv: response", "args", args, "data", string(data))
		return data, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("mpv command timed out: %v", args)
	}
}

func (c *Client) LoadFile(path, mode string) error {
	_, err := c.Command("loadfile", path, mode)
	return err
}

func (c *Client) PlaylistNext() error  { _, err := c.Command("playlist-next", "weak"); return err }
func (c *Client) PlaylistPrev() error  { _, err := c.Command("playlist-prev", "weak"); return err }
func (c *Client) PlaylistClear() error { _, err := c.Command("playlist-clear"); return err }
func (c *Client) PlaylistRemove(i int) error {
	_, err := c.Command("playlist-remove", i)
	return err
}
func (c *Client) PlaylistPlayIndex(i int) error {
	_, err := c.Command("set_property", "playlist-pos", i)
	return err
}
func (c *Client) Stop() error { _, err := c.Command("stop"); return err }

func (c *Client) SetPause(paused bool) error {
	_, err := c.Command("set_property", "pause", paused)
	return err
}

func (c *Client) Seek(seconds float64) error {
	_, err := c.Command("seek", seconds, "absolute")
	return err
}

func (c *Client) SetVolume(v float64) error {
	_, err := c.Command("set_property", "volume", v)
	return err
}

func (c *Client) Events() <-chan RawEvent { return c.events }

func (c *Client) Close() error {
	c.closed.Store(true)
	_, _ = c.Command("quit")
	_ = c.conn.Close()
	done := make(chan struct{})
	go func() { c.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
	}
	return nil
}

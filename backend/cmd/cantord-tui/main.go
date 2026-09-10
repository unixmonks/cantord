// Command cantord-tui is a full-screen, keyboard-driven client for cantord —
// a third frontend alongside the web UI and cantordctl, talking to the same
// HTTP/JSON + SSE API. Navigation is vim-style throughout (j/k, g/G, /
// to filter, dd to remove).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	server := os.Getenv("CANTORD_ADDR")
	if server == "" {
		server = "http://localhost:8080"
	}
	keysPath := os.Getenv("CANTORD_TUI_KEYS")
	flag.StringVar(&server, "server", server, "cantord daemon address")
	flag.StringVar(&keysPath, "keys", keysPath, "path to a TOML file of keyboard shortcut overrides (optional)")
	flag.Parse()

	keyCfg, err := LoadKeyConfig(keysPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cantord-tui:", err)
		os.Exit(1)
	}
	applyKeyConfig(keyCfg)

	client := NewClient(server)

	events := make(chan tea.Msg, 8)
	raw := make(chan sseEvent)
	go func() {
		for ev := range raw {
			events <- sseMsg(ev)
		}
	}()
	go func() {
		ctx := context.Background()
		backoff := time.Second
		for {
			start := time.Now()
			err := client.StreamEvents(ctx, raw)
			_ = err
			events <- connLostMsg{}
			// A connection that lasted a while was healthy, not a repeat
			// failure — don't let one old drop keep every future retry
			// waiting the full backed-off delay. The daemon's own SSE
			// "status" priming event flips the footer back to connected
			// as soon as a retry actually succeeds, so there's no need to
			// (and no reliable way to) claim "restored" from here.
			if time.Since(start) > 5*time.Second {
				backoff = time.Second
			}
			time.Sleep(backoff)
			if backoff < 15*time.Second {
				backoff *= 2
			}
		}
	}()

	m := newModel(client, events)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cantord-tui:", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// aiPageSize is roughly how many lines pgup/pgdown scroll the transcript by.
const aiPageSize = 10

// aiRole distinguishes a transcript line's speaker for rendering.
type aiRole int

const (
	aiRoleUser aiRole = iota
	aiRoleAssistant
	aiRoleStatus // a tool call/result or error, rendered dim and un-prefixed
)

type aiLine struct {
	role aiRole
	text string
}

// aiOverlay is the "ctrl+a" AI assistant chat popup, reachable from any tab.
// Unlike searchOverlay and themePicker, closing it (esc) doesn't discard its
// state — only open flips to false — so reopening resumes the same
// conversation and scrollback instead of starting over, and a reply already
// streaming in keeps arriving while the popup is hidden.
type aiOverlay struct {
	open  bool
	input textinput.Model

	// configured mirrors Client.AiStatus at the time the overlay was first
	// opened — checked once up front so the empty state can say plainly
	// that CANTORD_AI_API_KEY isn't set, instead of the user having to send
	// a message just to learn that.
	configured bool

	convID   string
	messages []aiLine

	streaming bool
	cancel    context.CancelFunc
	events    chan sseEvent

	// needsQueueRefresh is set when a tool_result replaces the queue, so
	// the Queue tab's screen is reloaded once the turn finishes rather than
	// mid-stream.
	needsQueueRefresh bool

	scrollOffset int
}

func newAiOverlay(configured bool) *aiOverlay {
	ti := textinput.New()
	ti.Placeholder = "ask for something like \"play me 90s grunge\"…"
	ti.CharLimit = 500
	ti.Width = 60
	ti.Focus()
	return &aiOverlay{open: true, input: ti, configured: configured}
}

// --- messages / commands ---

// aiStatusMsg carries the daemon's /api/ai/status check made once at
// startup, so the overlay's very first open already knows whether the
// assistant is configured rather than finding out only after a wasted
// message.
type aiStatusMsg struct{ configured bool }

func loadAiStatus(c *Client) tea.Cmd {
	return func() tea.Msg {
		st, err := c.AiStatus()
		return aiStatusMsg{configured: err == nil && st.Configured}
	}
}

// aiChatEventMsg carries one decoded SSE frame from an in-flight turn, or
// ok=false once its channel closes (the turn's HTTP request has ended).
type aiChatEventMsg struct {
	event sseEvent
	ok    bool
}

func waitForAiEvent(ch <-chan sseEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return aiChatEventMsg{event: ev, ok: ok}
	}
}

// startAiTurn sends message as the next turn of ov's conversation and
// starts streaming the reply in via a background goroutine — bubbletea
// commands only ever return one message, so unlike a one-shot request
// (cmdSearch) this needs its own long-lived channel, the same pattern
// main.go uses for the daemon-wide SSE feed.
func (m *Model) startAiTurn(message string) tea.Cmd {
	ov := m.aiOverlay
	ctx, cancel := context.WithCancel(context.Background())
	ov.cancel = cancel
	ch := make(chan sseEvent, 8)
	ov.events = ch

	c := m.client
	convID := ov.convID
	go func() {
		defer close(ch)
		if err := c.AiChat(ctx, convID, message, ch); err != nil && ctx.Err() == nil {
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			ch <- sseEvent{name: "error", data: string(data)}
		}
	}()
	return waitForAiEvent(ch)
}

// appendAssistantText appends a text_delta chunk to the conversation,
// coalescing consecutive deltas into one growing assistant message rather
// than one aiLine per chunk.
func (ov *aiOverlay) appendAssistantText(text string) {
	if n := len(ov.messages); n > 0 && ov.messages[n-1].role == aiRoleAssistant {
		ov.messages[n-1].text += text
		return
	}
	ov.messages = append(ov.messages, aiLine{role: aiRoleAssistant, text: text})
}

// handleAiEvent applies one decoded SSE frame to ov's transcript. Returns a
// tea.Cmd only for "done", to refresh the Queue tab if this turn changed it.
func (m *Model) handleAiEvent(ev sseEvent) tea.Cmd {
	ov := m.aiOverlay
	switch ev.name {
	case "conversation":
		var data struct {
			ConversationID string `json:"conversation_id"`
		}
		json.Unmarshal([]byte(ev.data), &data)
		ov.convID = data.ConversationID

	case "text_delta":
		var text string
		if json.Unmarshal([]byte(ev.data), &text) == nil {
			ov.appendAssistantText(text)
		}

	case "tool_call":
		var data struct {
			Name string `json:"name"`
		}
		json.Unmarshal([]byte(ev.data), &data)
		ov.messages = append(ov.messages, aiLine{role: aiRoleStatus, text: aiToolCallLabel(data.Name)})

	case "tool_result":
		var data struct {
			Name    string `json:"name"`
			IsError bool   `json:"is_error"`
			Result  struct {
				Message string `json:"message"`
			} `json:"result"`
		}
		json.Unmarshal([]byte(ev.data), &data)
		mark := "✓ "
		if data.IsError {
			mark = "✗ "
		} else if data.Name == "set_queue" {
			ov.needsQueueRefresh = true
		}
		ov.messages = append(ov.messages, aiLine{role: aiRoleStatus, text: mark + data.Result.Message})

	case "done":
		ov.streaming = false
		if ov.needsQueueRefresh {
			ov.needsQueueRefresh = false
			if qs := m.queueScreen(); qs != nil {
				return loadQueue(m.client, qs.id)
			}
		}

	case "error":
		var data struct {
			Error string `json:"error"`
		}
		json.Unmarshal([]byte(ev.data), &data)
		ov.streaming = false
		ov.messages = append(ov.messages, aiLine{role: aiRoleStatus, text: "✗ " + data.Error})
	}
	return nil
}

// aiToolCallLabel is the transient "working on it" line shown the moment a
// tool call starts, before its tool_result (with the real outcome) lands.
func aiToolCallLabel(name string) string {
	switch name {
	case "search_library":
		return "→ searching library…"
	case "list_genres":
		return "→ checking genres…"
	case "get_queue":
		return "→ checking queue…"
	case "get_favorites":
		return "→ checking favorites…"
	case "set_queue":
		return "→ updating queue…"
	case "create_playlist":
		return "→ creating playlist…"
	case "add_to_playlist":
		return "→ updating playlist…"
	default:
		return "→ " + name + "…"
	}
}

// --- Model integration ---

func (m *Model) toggleAiChat() tea.Cmd {
	if m.aiOverlay == nil {
		m.aiOverlay = newAiOverlay(m.aiConfigured)
		return textinput.Blink
	}
	m.aiOverlay.open = !m.aiOverlay.open
	if m.aiOverlay.open {
		m.aiOverlay.input.Focus()
		return textinput.Blink
	}
	m.aiOverlay.input.Blur()
	return nil
}

// handleAiKey is handleKey's dispatch while the AI overlay is open. Unlike
// the search overlay there's no separate navigate mode: the input is always
// live, since there's nothing here to move a cursor over.
func (m Model) handleAiKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ov := m.aiOverlay
	switch msg.String() {
	case "esc":
		if ov.streaming && ov.cancel != nil {
			ov.cancel()
			ov.streaming = false
		}
		ov.open = false
		return m, nil

	case "enter":
		if ov.streaming {
			return m, nil
		}
		text := strings.TrimSpace(ov.input.Value())
		if text == "" {
			return m, nil
		}
		ov.input.SetValue("")
		ov.messages = append(ov.messages, aiLine{role: aiRoleUser, text: text})
		ov.scrollOffset = 0
		ov.streaming = true
		return m, m.startAiTurn(text)

	case "pgup":
		ov.scrollOffset += aiPageSize
		return m, nil
	case "pgdown":
		ov.scrollOffset = max(ov.scrollOffset-aiPageSize, 0)
		return m, nil

	default:
		var cmd tea.Cmd
		ov.input, cmd = ov.input.Update(msg)
		return m, cmd
	}
}

// --- rendering ---

// aiBoxOverhead is everything renderAiOverlay's promptBoxStyle box adds
// above/below the scrollable transcript: a rounded border (top+bottom) and
// 1-line vertical padding on each side (4 rows), plus the title, a blank
// line, a blank separator before the input, the input line, and the mode
// hint (5 rows).
const aiBoxOverhead = 9

func aiHint(ov *aiOverlay) string {
	if ov.streaming {
		return "waiting for reply… · esc to cancel"
	}
	return "enter to send · esc to close"
}

// buildAiLines word-wraps every message in ov to innerWidth and flattens
// them into one line-per-row list, ready to be windowed by height.
func buildAiLines(ov *aiOverlay, innerWidth int) []string {
	var lines []string
	for i, msg := range ov.messages {
		if i > 0 {
			lines = append(lines, "")
		}
		switch msg.role {
		case aiRoleStatus:
			lines = append(lines, footerLabelStyle.Render(msg.text))
		case aiRoleUser:
			wrapped := lipgloss.NewStyle().Width(innerWidth - 2).Render(msg.text)
			for j, l := range strings.Split(wrapped, "\n") {
				if j == 0 {
					lines = append(lines, searchSelectedStyle.Render("› ")+l)
				} else {
					lines = append(lines, "  "+l)
				}
			}
		default: // aiRoleAssistant
			wrapped := lipgloss.NewStyle().Width(innerWidth).Render(msg.text)
			lines = append(lines, strings.Split(wrapped, "\n")...)
		}
	}
	return lines
}

// renderAiOverlay never lets its box grow taller than height: the
// transcript is windowed around the bottom (or ov.scrollOffset lines above
// it) and scrolls, the same way renderSearchOverlay windows around its
// cursor.
func renderAiOverlay(width, height int, ov *aiOverlay) string {
	boxWidth := min(max(width-4, 20), 90)
	innerWidth := max(boxWidth-6, 10)

	lines := []string{listTitleStyle.Render("AI Assistant"), ""}

	switch {
	case len(ov.messages) == 0 && !ov.configured:
		lines = append(lines, errorStyle.Render("AI assistant not configured — set CANTORD_AI_API_KEY and restart cantord"))
	case len(ov.messages) == 0:
		lines = append(lines, footerLabelStyle.Render("ask it to play or make a playlist of something"))
	default:
		all := buildAiLines(ov, innerWidth)
		viewHeight := max(height-aiBoxOverhead, 3)
		maxTop := max(len(all)-viewHeight, 0)
		top := min(max(maxTop-ov.scrollOffset, 0), maxTop)
		bottom := min(top+viewHeight, len(all))
		lines = append(lines, all[top:bottom]...)
	}

	lines = append(lines, "", ov.input.View(), footerLabelStyle.Render(aiHint(ov)))

	for i, l := range lines {
		lines[i] = truncateLine(l, innerWidth)
	}

	box := promptBoxStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

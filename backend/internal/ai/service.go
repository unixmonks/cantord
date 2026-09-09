package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"cantord/internal/library"
	"cantord/internal/playback"
)

const defaultModel = "claude-opus-5"

// maxToolIterations bounds one chat turn's tool-calling loop — a request
// wall the model can't ever get the daemon to exceed even if it keeps
// asking for more tool calls.
const maxToolIterations = 6

const systemPrompt = `You are cantord's music assistant, built into a local music player. You can search the user's actual music library and act on it — build a play queue or save a playlist — using the tools provided. You do not have any other capabilities: no web access, no knowledge of what's NOT in this library beyond your own training knowledge of music.

Guidelines:
- Requests like "play me X" mean: search for fitting tracks, then call set_queue. Requests to "make/create a playlist of X" mean: search, then call create_playlist.
- Use get_queue to see what's currently queued/playing — before replacing the queue for a request that implies adding to or building on what's already there ("add some X to this", "what's next", "queue up more like this"), and to answer any question about the current queue.
- Track metadata in real libraries is messy — genre is a single inconsistent tag and there's no "decade" or "popularity" field. Use your own knowledge of artists, songs, and eras to decide what fits a request (e.g. "90s grunge" -> Nirvana, Alice in Chains, Soundgarden, Pearl Jam, Stone Temple Pilots...), then use search_library (by artist name, or a text query, or genre if list_genres shows a fitting tag) to find which of those the user actually owns. Never invent a track_id — only use ones a tool returned.
- Pick a reasonable number of tracks for the request (roughly 10-20 for an open-ended "play me" request) unless the user asked for a specific count or "everything by X".
- After taking an action (set_queue/create_playlist/add_to_playlist), reply with one short confirming sentence — don't repeat the full track list, the UI already shows it.
- If nothing in the library matches, say so plainly rather than queuing an empty or unrelated result.`

// Service runs chat turns against Claude with the tool set defined in
// tools.go, streaming text and tool activity out via an EventFunc while
// persisting the transcript through library.Store.
type Service struct {
	client     anthropic.Client
	model      string
	configured bool
	lib        *library.Store
	engine     *playback.Engine
	tools      []Tool
}

// New builds the assistant. apiKey may be empty — in that case Configured()
// reports false and Chat refuses to run, but the daemon still starts
// normally (the AI feature is optional).
func New(apiKey, model string, lib *library.Store, engine *playback.Engine) *Service {
	if model == "" {
		model = defaultModel
	}
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &Service{
		client:     anthropic.NewClient(opts...),
		model:      model,
		configured: apiKey != "",
		lib:        lib,
		engine:     engine,
		tools:      tools(),
	}
}

func (s *Service) Configured() bool { return s.configured }
func (s *Service) Model() string    { return s.model }

func (s *Service) toolParams() []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(s.tools))
	for _, t := range s.tools {
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.InputSchema,
				Required:   t.Required,
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return out
}

func (s *Service) runTool(ctx context.Context, name string, input json.RawMessage) (ToolResult, error) {
	for _, t := range s.tools {
		if t.Name == name {
			return t.Run(ctx, &Deps{Lib: s.lib, Engine: s.engine}, input)
		}
	}
	return ToolResult{}, fmt.Errorf("unknown tool %q", name)
}

// Chat runs one user turn to completion: it loads the conversation's prior
// transcript, sends the new message plus tool definitions, executes any
// tool calls Claude makes (looping until it produces a final answer or
// maxToolIterations is hit), and emits Events for the HTTP layer to stream
// out as SSE. The final assistant reply is persisted before Chat returns.
func (s *Service) Chat(ctx context.Context, conversationID, userMessage string, emit EventFunc) error {
	if !s.configured {
		return fmt.Errorf("AI assistant is not configured (set CANTORD_AI_API_KEY)")
	}

	history, err := s.lib.ListAiMessages(conversationID)
	if err != nil {
		return err
	}
	if err := s.lib.SaveAiMessage(conversationID, "user", userMessage); err != nil {
		return err
	}

	messages := make([]anthropic.MessageParam, 0, len(history)+1)
	for _, m := range history {
		if m.Role == "assistant" {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		} else {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)))

	toolParams := s.toolParams()
	system := []anthropic.TextBlockParam{{Text: systemPrompt}}

	var finalText strings.Builder
	for i := 0; i < maxToolIterations; i++ {
		stream := s.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(s.model),
			MaxTokens: 4096,
			System:    system,
			Messages:  messages,
			Tools:     toolParams,
		})

		message := anthropic.Message{}
		finalText.Reset()
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				return err
			}
			if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok && text.Text != "" {
					finalText.WriteString(text.Text)
					emit(Event{Type: "text_delta", Data: text.Text})
				}
			}
		}
		if err := stream.Err(); err != nil {
			// Returned, not emitted here — the HTTP handler is the single
			// place that turns a returned error into the "error" SSE frame,
			// so every failure path (this one, a config/store error before
			// the stream even starts) produces exactly one such event.
			return err
		}

		messages = append(messages, message.ToParam())

		if message.StopReason != anthropic.StopReasonToolUse {
			break
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			tu, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			emit(Event{Type: "tool_call", Data: map[string]any{"name": tu.Name, "input": json.RawMessage(tu.Input)}})

			result, err := s.runTool(ctx, tu.Name, tu.Input)
			isErr := err != nil
			resultJSON := result.json()
			if err != nil {
				resultJSON = err.Error()
				slog.Warn("ai: tool call failed", "tool", tu.Name, "err", err)
			}
			emit(Event{Type: "tool_result", Data: map[string]any{"name": tu.Name, "result": result, "is_error": isErr}})
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, resultJSON, isErr))
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	reply := strings.TrimSpace(finalText.String())
	if reply == "" {
		reply = "Done."
	}
	if err := s.lib.SaveAiMessage(conversationID, "assistant", reply); err != nil {
		slog.Warn("ai: saving assistant reply", "err", err)
	}
	emit(Event{Type: "done", Data: map[string]any{"text": reply}})
	return nil
}

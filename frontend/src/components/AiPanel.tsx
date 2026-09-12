import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { AiStatus, AiToolCall, AiToolResult } from "../api/types";
import { IconClose, IconSend, IconSparkle } from "./Icons";

type ChatItem =
  | { id: number; kind: "user"; text: string }
  | { id: number; kind: "assistant"; text: string }
  | { id: number; kind: "tool"; name: string; status: "running" | "done" | "error"; result?: AiToolResult; errorText?: string };

// Tools the model calls purely to look something up before deciding what
// to do — worth a quiet status line, not a prominent card. set_queue /
// create_playlist / add_to_playlist actually change the user's queue or
// playlists, so those get the confirmation-card treatment.
const LOOKUP_TOOLS = new Set(["search_library", "list_genres", "get_favorites", "get_queue"]);

const RUNNING_LABEL: Record<string, string> = {
  search_library: "Searching your library…",
  list_genres: "Checking genres…",
  get_favorites: "Checking favorites…",
  get_queue: "Checking the queue…",
  set_queue: "Updating the queue…",
  create_playlist: "Creating playlist…",
  add_to_playlist: "Updating playlist…",
};

export function AiPanel() {
  const { api, aiPanelOpen, toggleAiPanel, aiPrompt, clearAiPrompt, showToast } = usePlayer();
  const [status, setStatus] = useState<AiStatus | null>(null);
  const [items, setItems] = useState<ChatItem[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const conversationId = useRef<string | undefined>(undefined);
  const idCounter = useRef(0);
  const pendingToolId = useRef<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!aiPanelOpen) return;
    api.aiStatus().then(setStatus).catch(() => setStatus({ configured: false, provider: "", model: "" }));
  }, [aiPanelOpen, api]);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [items]);

  useEffect(() => {
    return () => abortRef.current?.abort();
  }, []);

  // A quick link on the home screen routes here through PlayerContext
  // instead of calling the chat API directly, so it runs as a real chat
  // turn — same streaming reply and tool-call cards a typed message gets.
  useEffect(() => {
    if (!aiPrompt) return;
    send(aiPrompt.text);
    clearAiPrompt();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [aiPrompt]);

  function nextId() {
    return ++idCounter.current;
  }

  function newChat() {
    abortRef.current?.abort();
    conversationId.current = undefined;
    setItems([]);
    setSending(false);
  }

  async function send(overrideText?: string) {
    const message = (overrideText ?? input).trim();
    if (!message || sending) return;
    if (overrideText === undefined) setInput("");
    setSending(true);
    setItems((prev) => [...prev, { id: nextId(), kind: "user", text: message }]);

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await api.aiChat(message, conversationId.current, (ev) => {
        switch (ev.type) {
          case "conversation":
            conversationId.current = (ev.data as { conversation_id: string }).conversation_id;
            break;
          case "text_delta": {
            const text = ev.data as string;
            setItems((prev) => {
              const last = prev[prev.length - 1];
              if (last?.kind === "assistant") {
                return [...prev.slice(0, -1), { ...last, text: last.text + text }];
              }
              return [...prev, { id: nextId(), kind: "assistant", text }];
            });
            break;
          }
          case "tool_call": {
            const call = ev.data as AiToolCall;
            const id = nextId();
            pendingToolId.current = id;
            setItems((prev) => [...prev, { id, kind: "tool", name: call.name, status: "running" }]);
            break;
          }
          case "tool_result": {
            const data = ev.data as { result: AiToolResult; is_error: boolean };
            const id = pendingToolId.current;
            setItems((prev) =>
              prev.map((item) =>
                item.id === id && item.kind === "tool"
                  ? {
                      ...item,
                      status: data.is_error ? "error" : "done",
                      result: data.result,
                      errorText: data.is_error ? String(data.result?.message ?? "Failed") : undefined,
                    }
                  : item,
              ),
            );
            break;
          }
          case "error":
            showToast((ev.data as { error: string })?.error ?? "The assistant hit an error");
            break;
        }
      }, controller.signal);
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Could not reach the assistant");
    } finally {
      setSending(false);
    }
  }

  return (
    <div
      className="ai-panel"
      style={{
        position: "fixed",
        top: 0,
        bottom: 88,
        right: 0,
        width: 380,
        maxWidth: "100vw",
        background: "var(--bg-elev)",
        borderLeft: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        transform: aiPanelOpen ? "translateX(0)" : "translateX(100%)",
        transition: "transform 0.18s ease",
        zIndex: 40,
        boxShadow: aiPanelOpen ? "-8px 0 24px rgba(0,0,0,0.25)" : "none",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "16px 16px 12px",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <IconSparkle size={20} style={{ color: "var(--accent)" }} />
          <span className="disp" style={{ fontSize: 17, fontWeight: 700 }}>
            Assistant
          </span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          {items.length > 0 && (
            <button className="iconbtn" onClick={newChat} title="New chat" style={{ fontSize: 13, padding: "4px 8px", width: "auto" }}>
              New
            </button>
          )}
          <button className="iconbtn" onClick={toggleAiPanel} title="Close">
            <IconClose size={18} />
          </button>
        </div>
      </div>

      {status && !status.configured ? (
        <div style={{ padding: 20, color: "var(--text-dim)", fontSize: 14, lineHeight: 1.6 }}>
          The assistant isn't configured yet. Set <code>CANTORD_AI_API_KEY</code> to an Anthropic API key
          and restart cantord to enable it.
        </div>
      ) : (
        <>
          <div ref={scrollRef} style={{ flex: 1, overflowY: "auto", padding: 16, display: "flex", flexDirection: "column", gap: 10 }}>
            {items.length === 0 && (
              <div style={{ color: "var(--text-faint)", fontSize: 14, lineHeight: 1.6 }}>
                Try "play me 90s grunge hits" or "make a playlist of chill Sunday morning music".
              </div>
            )}
            {items.map((item) => <ChatItemView key={item.id} item={item} />)}
          </div>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              send();
            }}
            style={{ display: "flex", gap: 8, padding: 12, borderTop: "1px solid var(--border)", flexShrink: 0 }}
          >
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Ask for a queue or playlist…"
              disabled={sending}
              style={{
                flex: 1,
                background: "var(--bg-elev-2)",
                border: "1px solid var(--border)",
                borderRadius: 8,
                padding: "9px 12px",
                color: "var(--text)",
                fontSize: 14,
              }}
            />
            <button
              type="submit"
              className="iconbtn"
              disabled={sending || !input.trim()}
              title="Send"
              style={{ opacity: sending || !input.trim() ? 0.4 : 1 }}
            >
              <IconSend size={18} />
            </button>
          </form>
        </>
      )}
    </div>
  );
}

function ChatItemView({ item }: { item: ChatItem }) {
  if (item.kind === "user") {
    return (
      <div
        style={{
          alignSelf: "flex-end",
          maxWidth: "85%",
          background: "var(--accent)",
          color: "var(--accent-ink)",
          borderRadius: 10,
          padding: "8px 12px",
          fontSize: 14,
          lineHeight: 1.5,
        }}
      >
        {item.text}
      </div>
    );
  }

  if (item.kind === "assistant") {
    if (!item.text) return null;
    return (
      <div
        style={{
          alignSelf: "flex-start",
          maxWidth: "90%",
          background: "var(--bg-elev-2)",
          borderRadius: 10,
          padding: "8px 12px",
          fontSize: 14,
          lineHeight: 1.5,
          whiteSpace: "pre-wrap",
        }}
      >
        {item.text}
      </div>
    );
  }

  // tool item
  if (LOOKUP_TOOLS.has(item.name)) {
    return (
      <div style={{ alignSelf: "flex-start", fontSize: 13, color: "var(--text-faint)", display: "flex", alignItems: "center", gap: 6 }}>
        <span>{item.status === "running" ? "⟳" : item.status === "error" ? "✕" : "✓"}</span>
        <span>{item.status === "running" ? RUNNING_LABEL[item.name] ?? "Working…" : item.result?.message}</span>
      </div>
    );
  }

  return (
    <div
      style={{
        alignSelf: "flex-start",
        maxWidth: "90%",
        background: "var(--bg-elev-2)",
        border: "1px solid var(--border)",
        borderRadius: 10,
        padding: "10px 12px",
        fontSize: 14,
      }}
    >
      {item.status === "running" && <div style={{ color: "var(--text-dim)" }}>{RUNNING_LABEL[item.name] ?? "Working…"}</div>}
      {item.status === "error" && <div style={{ color: "var(--danger)" }}>{item.errorText}</div>}
      {item.status === "done" && item.result && (
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <div style={{ fontWeight: 600 }}>{item.result.message}</div>
          {item.name === "set_queue" && (
            <Link to="/queue" style={{ color: "var(--accent-2)", fontSize: 13 }}>
              View queue →
            </Link>
          )}
          {(item.name === "create_playlist" || item.name === "add_to_playlist") && item.result.playlist_name && (
            <Link to={`/playlists/${encodeURIComponent(item.result.playlist_name)}`} style={{ color: "var(--accent-2)", fontSize: 13 }}>
              Open playlist →
            </Link>
          )}
        </div>
      )}
    </div>
  );
}

import { useEffect, useRef, useState } from "react";
import { usePlayer } from "../state/PlayerContext";
import { IconCheck, IconPlaylist, IconPlus } from "./Icons";

interface AddToPlaylistMenuProps {
  trackIds: string[];
  variant?: "icon" | "button";
  label?: string;
  disabled?: boolean;
}

// Icon or button that opens a small dropdown for adding one or more tracks
// to an existing playlist (or a brand new one). Used from track rows, album
// pages, and the queue — anywhere a set of track ids is already at hand.
export function AddToPlaylistMenu({ trackIds, variant = "icon", label = "Add to playlist", disabled = false }: AddToPlaylistMenuProps) {
  const { api, showToast } = usePlayer();
  const [open, setOpen] = useState(false);
  const [playlists, setPlaylists] = useState<string[] | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    api.listPlaylists().then(setPlaylists).catch(() => setPlaylists([]));
  }, [open, api]);

  useEffect(() => {
    if (!open) return;
    function onOutside(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) close();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") close();
    }
    document.addEventListener("mousedown", onOutside);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onOutside);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  function close() {
    setOpen(false);
    setCreating(false);
    setNewName("");
  }

  async function addTo(name: string) {
    if (busy) return;
    setBusy(true);
    try {
      for (const id of trackIds) await api.addToPlaylist(name, id);
      showToast(`Added ${trackIds.length === 1 ? "track" : `${trackIds.length} tracks`} to "${name}"`);
      close();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't add to playlist");
    } finally {
      setBusy(false);
    }
  }

  function createAndAdd() {
    const name = newName.trim();
    if (!name) return;
    addTo(name);
  }

  return (
    <div ref={rootRef} style={{ position: "relative", flexShrink: 0 }} onClick={(e) => e.stopPropagation()}>
      {variant === "icon" ? (
        <button className="iconbtn" disabled={disabled} onClick={() => setOpen((v) => !v)} title={label}>
          <IconPlaylist size={17} />
        </button>
      ) : (
        <button className="btn" disabled={disabled} onClick={() => setOpen((v) => !v)}>
          <IconPlaylist size={16} /> {label}
        </button>
      )}

      {open && (
        <div
          style={{
            position: "absolute",
            top: "100%",
            right: 0,
            marginTop: 4,
            width: 220,
            maxHeight: 280,
            overflowY: "auto",
            background: "var(--bg-elev-2)",
            border: "1px solid var(--border)",
            borderRadius: 10,
            boxShadow: "0 8px 24px rgba(0,0,0,0.4)",
            zIndex: 100,
            padding: 6,
          }}
        >
          {playlists === null && <div style={{ padding: "10px 12px", fontSize: 14, color: "var(--text-faint)" }}>Loading…</div>}

          {playlists?.length === 0 && !creating && (
            <div style={{ padding: "10px 12px", fontSize: 14, color: "var(--text-faint)" }}>No playlists yet.</div>
          )}

          {playlists?.map((name) => (
            <button
              key={name}
              disabled={busy}
              onClick={() => addTo(name)}
              className="menu-item"
              style={{
                display: "flex",
                width: "100%",
                alignItems: "center",
                gap: 8,
                padding: "9px 10px",
                fontSize: 14.5,
                color: "var(--text)",
                background: "none",
                border: "none",
                borderRadius: 6,
                cursor: "pointer",
                textAlign: "left",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {name}
            </button>
          ))}

          <div style={{ borderTop: playlists && playlists.length > 0 ? "1px solid var(--border)" : "none", marginTop: playlists && playlists.length > 0 ? 4 : 0, paddingTop: playlists && playlists.length > 0 ? 4 : 0 }}>
            {creating ? (
              <div style={{ display: "flex", gap: 4, padding: 4 }}>
                <input
                  autoFocus
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && createAndAdd()}
                  placeholder="Playlist name…"
                  className="field"
                  style={{ flex: 1, fontSize: 14, padding: "6px 8px" }}
                />
                <button className="iconbtn" disabled={busy || !newName.trim()} onClick={createAndAdd} title="Create">
                  <IconCheck size={16} />
                </button>
              </div>
            ) : (
              <button
                onClick={() => setCreating(true)}
                className="menu-item"
                style={{
                  display: "flex",
                  width: "100%",
                  alignItems: "center",
                  gap: 8,
                  padding: "9px 10px",
                  fontSize: 14.5,
                  color: "var(--text-dim)",
                  background: "none",
                  border: "none",
                  borderRadius: 6,
                  cursor: "pointer",
                  textAlign: "left",
                }}
              >
                <IconPlus size={15} /> New playlist
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

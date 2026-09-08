import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import { IconPlaylist, IconPlay, IconTrash } from "../components/Icons";

export function PlaylistsPage() {
  const { api, queue, showToast } = usePlayer();
  const navigate = useNavigate();
  const [playlists, setPlaylists] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [newName, setNewName] = useState("");
  const [saving, setSaving] = useState(false);

  function refresh() {
    setLoading(true);
    api
      .listPlaylists()
      .then(setPlaylists)
      .finally(() => setLoading(false));
  }

  useEffect(refresh, [api]);

  async function saveQueueAsPlaylist() {
    const name = newName.trim();
    if (!name) return;
    if (queue.length === 0) {
      showToast("Queue is empty — nothing to save");
      return;
    }
    setSaving(true);
    try {
      await api.savePlaylist(name, queue.map((t) => t.id));
      setNewName("");
      showToast(`Saved "${name}"`);
      refresh();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't save playlist");
    } finally {
      setSaving(false);
    }
  }

  async function playAll(name: string) {
    try {
      const { track_ids } = await api.getPlaylist(name);
      const startIndex = queue.length;
      for (const id of track_ids) await api.enqueue(id);
      await api.playIndex(startIndex);
      showToast(`Playing "${name}"`);
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't play playlist");
    }
  }

  async function deletePlaylist(name: string) {
    if (!window.confirm(`Delete playlist "${name}"? This can't be undone.`)) return;
    try {
      await api.deletePlaylist(name);
      refresh();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't delete playlist");
    }
  }

  if (loading) return <div className="empty-state">Loading playlists…</div>;

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          Playlists
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>
          {playlists.length} saved playlist{playlists.length === 1 ? "" : "s"}
        </div>
      </div>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          background: "var(--bg-elev)",
          border: "1px solid var(--border)",
          borderRadius: 10,
          padding: "8px 8px 8px 16px",
          maxWidth: 520,
          marginBottom: 36,
        }}
      >
        <IconPlaylist size={16} style={{ color: "var(--text-faint)", flexShrink: 0 }} />
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && saveQueueAsPlaylist()}
          placeholder="Save current queue as playlist…"
          style={{ flex: 1, background: "none", border: "none", outline: "none", color: "var(--text)", fontSize: 14 }}
        />
        <button className="btn btn-primary" disabled={saving || !newName.trim()} onClick={saveQueueAsPlaylist}>
          Save
        </button>
      </div>

      {playlists.length === 0 ? (
        <div className="empty-state">No playlists yet — build a queue and save it above.</div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 2, maxWidth: 640 }}>
          {playlists.map((name) => (
            <div
              key={name}
              className="row-hover"
              style={{ display: "flex", alignItems: "center", gap: 16, padding: 14, cursor: "pointer" }}
              onClick={() => navigate(`/playlists/${encodeURIComponent(name)}`)}
            >
              <div
                style={{
                  width: 48,
                  height: 48,
                  borderRadius: 8,
                  background: "var(--bg-elev-2)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                  color: "var(--text-dim)",
                }}
              >
                <IconPlaylist size={20} />
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 15, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{name}</div>
              </div>
              <button
                className="iconbtn"
                onClick={(e) => {
                  e.stopPropagation();
                  playAll(name);
                }}
                title="Play all"
              >
                <IconPlay size={16} />
              </button>
              <button
                className="iconbtn"
                onClick={(e) => {
                  e.stopPropagation();
                  deletePlaylist(name);
                }}
                title="Delete playlist"
              >
                <IconTrash size={16} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

import { useEffect, useState, type DragEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Track } from "../api/types";
import { TrackRow } from "../components/TrackRow";
import { IconBack, IconPlay, IconPlus, IconTrash } from "../components/Icons";
import { formatDurationLong } from "../utils/format";

export function PlaylistDetailPage() {
  const { name: routeName } = useParams<{ name: string }>();
  const { api, queue, showToast } = usePlayer();
  const navigate = useNavigate();
  // Always up to the playlist list, not wherever in-app history happened to
  // come from — unlike albums/artists, a playlist has no natural "came from
  // a related entity" case (e.g. the AI assistant links into a playlist
  // from anywhere), so history-based back would land somewhere unrelated.
  const goBack = () => navigate("/playlists");
  const [name, setName] = useState(routeName ?? "");
  const [tracks, setTracks] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingName, setEditingName] = useState(false);
  const [draftName, setDraftName] = useState(routeName ?? "");
  const [dragIndex, setDragIndex] = useState<number | null>(null);

  useEffect(() => {
    if (!routeName) return;
    setName(routeName);
    setDraftName(routeName);
    setLoading(true);
    api
      .getPlaylist(routeName)
      .then(async ({ track_ids }) => {
        const resolved = await Promise.all(
          track_ids.map((id) => api.getTrack(id).catch(() => null)),
        );
        setTracks(resolved.filter((t): t is Track => t !== null));
      })
      .catch((err) => showToast(err instanceof Error ? err.message : "Couldn't load playlist"))
      .finally(() => setLoading(false));
  }, [api, routeName, showToast]);

  async function persistOrder(next: Track[]) {
    setTracks(next);
    try {
      await api.savePlaylist(name, next.map((t) => t.id));
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't reorder playlist");
    }
  }

  function onDrop(index: number) {
    return (e: DragEvent) => {
      e.preventDefault();
      if (dragIndex === null || dragIndex === index) return;
      const next = [...tracks];
      const [moved] = next.splice(dragIndex, 1);
      next.splice(index, 0, moved);
      setDragIndex(null);
      persistOrder(next);
    };
  }

  async function removeTrack(track: Track) {
    try {
      await api.removeFromPlaylist(name, track.id);
      setTracks((prev) => prev.filter((t) => t.id !== track.id));
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't remove track");
    }
  }

  async function playAll() {
    try {
      const startIndex = queue.length;
      for (const t of tracks) await api.enqueue(t.id);
      await api.playIndex(startIndex);
      showToast("Playing playlist");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't play playlist");
    }
  }

  async function addAllToQueue() {
    try {
      for (const t of tracks) await api.enqueue(t.id);
      showToast("Added to queue");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't add to queue");
    }
  }

  async function deletePlaylist() {
    if (!window.confirm(`Delete playlist "${name}"? This can't be undone.`)) return;
    try {
      await api.deletePlaylist(name);
      navigate("/playlists");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't delete playlist");
    }
  }

  async function commitRename() {
    const next = draftName.trim();
    setEditingName(false);
    if (!next || next === name) {
      setDraftName(name);
      return;
    }
    try {
      await api.renamePlaylist(name, next);
      setName(next);
      navigate(`/playlists/${encodeURIComponent(next)}`, { replace: true });
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't rename playlist");
      setDraftName(name);
    }
  }

  const totalMs = tracks.reduce((sum, t) => sum + t.duration_ms, 0);

  return (
    <div>
      <div
        style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--text-faint)", fontSize: 13, marginBottom: 24, cursor: "pointer" }}
        onClick={goBack}
      >
        <IconBack size={14} /> Back
      </div>

      <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 12, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 8 }}>
            Playlist
          </div>
          {editingName ? (
            <input
              autoFocus
              value={draftName}
              onChange={(e) => setDraftName(e.target.value)}
              onBlur={commitRename}
              onKeyDown={(e) => e.key === "Enter" && commitRename()}
              className="field"
              style={{ fontSize: 28, fontWeight: 700, padding: "2px 8px", marginBottom: 10, width: 360 }}
            />
          ) : (
            <h1
              className="disp"
              style={{ margin: "0 0 10px", fontSize: 32, fontWeight: 700, cursor: "text" }}
              onClick={() => setEditingName(true)}
              title="Click to rename"
            >
              {name}
            </h1>
          )}
          <div style={{ fontSize: 13, color: "var(--text-faint)", marginBottom: 20 }}>
            {tracks.length} tracks{totalMs ? ` · ${formatDurationLong(totalMs)}` : ""}
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <button className="btn btn-primary" disabled={tracks.length === 0} onClick={playAll}>
              <IconPlay size={13} /> Play all
            </button>
            <button className="btn" disabled={tracks.length === 0} onClick={addAllToQueue}>
              <IconPlus size={14} /> Add all to queue
            </button>
          </div>
        </div>
        <button className="btn btn-danger" onClick={deletePlaylist}>
          <IconTrash size={14} /> Delete playlist
        </button>
      </div>

      {loading && <div className="empty-state">Loading…</div>}
      {!loading && tracks.length === 0 && <div className="empty-state">This playlist is empty.</div>}

      <div style={{ maxWidth: 760 }}>
        {tracks.map((track, i) => (
          <TrackRow
            key={`${track.id}-${i}`}
            track={track}
            index={i + 1}
            showArt={false}
            onChange={(updated) => setTracks((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))}
            onRemove={() => removeTrack(track)}
            draggable
            onDragStart={() => setDragIndex(i)}
            onDragOver={(e) => e.preventDefault()}
            onDrop={onDrop(i)}
            onDragEnd={() => setDragIndex(null)}
          />
        ))}
      </div>
    </div>
  );
}

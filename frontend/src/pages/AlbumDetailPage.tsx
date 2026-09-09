import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Album, Track } from "../api/types";
import { AlbumArt } from "../components/AlbumArt";
import { TrackRow } from "../components/TrackRow";
import { AddToPlaylistMenu } from "../components/AddToPlaylistMenu";
import { IconBack, IconPlay, IconPlus } from "../components/Icons";
import { formatDurationLong } from "../utils/format";
import { useBackNavigate } from "../hooks/useBackNavigate";

export function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { api, queue, playFromList, refreshQueue, showToast, libraryVersion, artUpdate } = usePlayer();
  const navigate = useNavigate();
  const goBack = useBackNavigate("/albums");
  const [album, setAlbum] = useState<Album | null>(null);
  const [tracks, setTracks] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([api.getAlbum(id), api.listAlbumTracks(id)])
      .then(([a, t]) => {
        setAlbum(a);
        setTracks(t);
      })
      .catch((err) => showToast(err instanceof Error ? err.message : "Couldn't load album"))
      .finally(() => setLoading(false));
  }, [api, id, libraryVersion, showToast]);

  useEffect(() => {
    if (!artUpdate) return;
    setAlbum((prev) => (prev && prev.id === artUpdate.albumId ? { ...prev, art_hash: artUpdate.artHash } : prev));
  }, [artUpdate]);

  async function enqueueAll(playFirst: boolean) {
    if (tracks.length === 0) return;
    setBusy(true);
    try {
      const startIndex = queue.length;
      for (const track of tracks) {
        await api.enqueue(track.id);
      }
      refreshQueue();
      if (playFirst) await api.playIndex(startIndex);
      showToast(playFirst ? "Playing album" : "Added album to queue");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't queue album");
    } finally {
      setBusy(false);
    }
  }

  const totalMs = tracks.reduce((sum, t) => sum + t.duration_ms, 0);

  if (loading) return <div className="empty-state">Loading album…</div>;
  if (!album) return <div className="empty-state">Album not found.</div>;

  return (
    <div>
      <div
        style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--text-faint)", fontSize: 13, marginBottom: 24, cursor: "pointer" }}
        onClick={goBack}
      >
        <IconBack size={14} /> Back
      </div>

      <div style={{ display: "flex", gap: 28, marginBottom: 32 }}>
        <AlbumArt artHash={album.art_hash} seed={album.id} label={album.name} size={180} radius={10} />
        <div style={{ display: "flex", flexDirection: "column", justifyContent: "flex-end" }}>
          <div style={{ fontSize: 12, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 8 }}>
            Album
          </div>
          <h1 className="disp" style={{ margin: "0 0 8px", fontSize: 36, fontWeight: 700 }}>
            {album.name}
          </h1>
          <div style={{ fontSize: 15, color: "var(--text-dim)", marginBottom: 4 }}>
            <span
              className="meta-link"
              style={{ cursor: "pointer" }}
              onClick={() => navigate(`/artists/${encodeURIComponent(album.album_artist)}`)}
            >
              {album.album_artist}
            </span>
            {album.year ? ` · ${album.year}` : ""}
          </div>
          <div style={{ fontSize: 13, color: "var(--text-faint)", marginBottom: 20 }}>
            {tracks.length} tracks · {formatDurationLong(totalMs)}
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <button className="btn btn-primary" disabled={busy} onClick={() => enqueueAll(true)}>
              <IconPlay size={13} /> Play album
            </button>
            <button className="btn" disabled={busy} onClick={() => enqueueAll(false)}>
              <IconPlus size={14} /> Add to queue
            </button>
            <AddToPlaylistMenu variant="button" trackIds={tracks.map((t) => t.id)} disabled={busy || tracks.length === 0} />
          </div>
        </div>
      </div>

      <div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 14,
            padding: "8px 10px",
            fontSize: 11,
            color: "var(--text-faint)",
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            borderBottom: "1px solid var(--border)",
            marginBottom: 4,
          }}
        >
          <div style={{ width: 20, textAlign: "center" }}>#</div>
          <div style={{ flex: 1 }}>Title</div>
        </div>
        {tracks.map((track, i) => (
          <TrackRow
            key={track.id}
            track={track}
            index={track.track_no || i + 1}
            showArt={false}
            showAlbum={false}
            showRating
            onPlay={() => playFromList(tracks, i)}
            onChange={(updated) => setTracks((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))}
          />
        ))}
      </div>
    </div>
  );
}

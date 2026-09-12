import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Track } from "../api/types";
import { TrackRow } from "../components/TrackRow";
import { AlbumArt } from "../components/AlbumArt";
import { IconBack } from "../components/Icons";
import { useBackNavigate } from "../hooks/useBackNavigate";

export function GenreDetailPage() {
  const { genre } = useParams<{ genre: string }>();
  const navigate = useNavigate();
  const { api, libraryVersion, showToast, playFromList, artUpdate } = usePlayer();
  const goBack = useBackNavigate("/genres");
  const [tracks, setTracks] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!genre) return;
    setLoading(true);
    api
      .tracksByGenre(genre)
      .then(setTracks)
      .catch((err) => showToast(err instanceof Error ? err.message : "Couldn't load genre"))
      .finally(() => setLoading(false));
  }, [api, genre, libraryVersion, showToast]);

  useEffect(() => {
    if (!artUpdate) return;
    setTracks((prev) => prev.map((t) => (t.album_id === artUpdate.albumId ? { ...t, art_hash: artUpdate.artHash } : t)));
  }, [artUpdate]);

  const artists = useMemo(() => {
    const names = new Set(tracks.map((t) => t.album_artist).filter(Boolean));
    return [...names].sort((a, b) => a.localeCompare(b));
  }, [tracks]);

  const albums = useMemo(() => {
    const byId = new Map<string, Track>();
    for (const t of tracks) if (!byId.has(t.album_id)) byId.set(t.album_id, t);
    return [...byId.values()].sort((a, b) => a.album.localeCompare(b.album));
  }, [tracks]);

  return (
    <div>
      <div
        style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--text-faint)", fontSize: 14, marginBottom: 24, cursor: "pointer" }}
        onClick={goBack}
      >
        <IconBack size={16} /> Back
      </div>

      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 32, fontWeight: 700 }}>
          {genre}
        </h1>
        <div style={{ fontSize: 14, color: "var(--text-faint)", marginTop: 4 }}>{tracks.length} tracks</div>
      </div>

      {loading && <div className="empty-state">Loading…</div>}
      {!loading && tracks.length === 0 && <div className="empty-state">No tracks tagged with this genre.</div>}

      {!loading && tracks.length > 0 && (
        <>
          {artists.length > 0 && (
            <div style={{ marginBottom: 36 }}>
              <div className="section-label" style={{ marginBottom: 6 }}>
                Artists · {artists.length}
              </div>
              <div style={{ maxWidth: 640 }}>
                {artists.map((name) => (
                  <div
                    key={name}
                    className="row-hover"
                    onClick={() => navigate(`/artists/${encodeURIComponent(name)}`)}
                    style={{ display: "flex", alignItems: "center", padding: "12px 14px", cursor: "pointer" }}
                  >
                    <span style={{ fontSize: 17, fontWeight: 500 }}>{name}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {albums.length > 0 && (
            <div style={{ marginBottom: 36 }}>
              <div className="section-label" style={{ marginBottom: 14 }}>
                Albums · {albums.length}
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 20, maxWidth: 900 }}>
                {albums.map((t) => (
                  <div
                    key={t.album_id}
                    className="row-hover"
                    onClick={() => navigate(`/albums/${t.album_id}`)}
                    style={{ display: "flex", alignItems: "center", gap: 14, padding: 10, cursor: "pointer" }}
                  >
                    <AlbumArt artHash={t.art_hash} seed={t.album_id} label={t.album} size={52} />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: 16, fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {t.album}
                      </div>
                      <div style={{ fontSize: 13, color: "var(--text-dim)" }}>
                        {t.album_artist}
                        {t.year ? ` · ${t.year}` : ""}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            <div className="section-label" style={{ marginBottom: 6 }}>
              Tracks · {tracks.length}
            </div>
            <div style={{ maxWidth: 720 }}>
              {tracks.map((track, i) => (
                <TrackRow
                  key={track.id}
                  track={track}
                  showRating
                  onPlay={() => playFromList(tracks, i)}
                  onChange={(updated) => setTracks((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))}
                />
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}

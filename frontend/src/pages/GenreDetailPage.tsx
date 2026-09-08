import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Track } from "../api/types";
import { TrackRow } from "../components/TrackRow";
import { IconBack } from "../components/Icons";
import { useBackNavigate } from "../hooks/useBackNavigate";

export function GenreDetailPage() {
  const { genre } = useParams<{ genre: string }>();
  const { api, libraryVersion, showToast, playTrackNow } = usePlayer();
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

  return (
    <div>
      <div
        style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--text-faint)", fontSize: 13, marginBottom: 24, cursor: "pointer" }}
        onClick={goBack}
      >
        <IconBack size={14} /> Back
      </div>

      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          {genre}
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>{tracks.length} tracks</div>
      </div>

      {loading && <div className="empty-state">Loading…</div>}
      {!loading && tracks.length === 0 && <div className="empty-state">No tracks tagged with this genre.</div>}

      <div style={{ maxWidth: 720 }}>
        {tracks.map((track) => (
          <TrackRow
            key={track.id}
            track={track}
            showRating
            onPlay={() => playTrackNow(track.id)}
            onChange={(updated) => setTracks((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))}
          />
        ))}
      </div>
    </div>
  );
}

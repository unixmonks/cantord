import { useEffect, useState } from "react";
import { usePlayer } from "../state/PlayerContext";
import type { Track } from "../api/types";
import { TrackRow } from "../components/TrackRow";

interface TrackListPageProps {
  title: string;
  emptyMessage: string;
  fetcher: (api: ReturnType<typeof usePlayer>["api"]) => Promise<Track[]>;
}

export function TrackListPage({ title, emptyMessage, fetcher }: TrackListPageProps) {
  const player = usePlayer();
  const { libraryVersion, showToast, playFromList } = player;
  const [tracks, setTracks] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    fetcher(player.api)
      .then(setTracks)
      .catch((err) => showToast(err instanceof Error ? err.message : `Couldn't load ${title.toLowerCase()}`))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [player.api, libraryVersion]);

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          {title}
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>{tracks.length} tracks</div>
      </div>

      {loading && <div className="empty-state">Loading…</div>}
      {!loading && tracks.length === 0 && <div className="empty-state">{emptyMessage}</div>}

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
  );
}

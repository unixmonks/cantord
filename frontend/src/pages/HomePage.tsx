import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Track } from "../api/types";
import { TrackRow } from "../components/TrackRow";
import { IconClock, IconHeartFilled } from "../components/Icons";

export function HomePage() {
  const { api, libraryVersion, showToast, playFromList } = usePlayer();
  const navigate = useNavigate();
  const [recent, setRecent] = useState<Track[]>([]);
  const [played, setPlayed] = useState<Track[]>([]);
  const [favorites, setFavorites] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([api.recentlyAdded(6), api.recentlyPlayed(6), api.listFavorites()])
      .then(([r, p, f]) => {
        setRecent(r);
        setPlayed(p);
        setFavorites(f.slice(0, 6));
      })
      .catch((err) => showToast(err instanceof Error ? err.message : "Couldn't load home"))
      .finally(() => setLoading(false));
  }, [api, libraryVersion, showToast]);

  if (loading) return <div className="empty-state">Loading…</div>;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 36 }}>
      <div>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          Home
        </h1>
      </div>

      <Section
        title="Recently Added"
        icon={<IconClock size={14} />}
        onSeeAll={() => navigate("/recent")}
        empty="Nothing has been added yet."
        tracks={recent}
        onPlay={(i) => playFromList(recent, i)}
      />
      <Section
        title="Recently Played"
        icon={<IconClock size={14} />}
        onSeeAll={() => navigate("/recently-played")}
        empty="Nothing has been played yet."
        tracks={played}
        onPlay={(i) => playFromList(played, i)}
      />
      <Section
        title="Favorites"
        icon={<IconHeartFilled size={14} />}
        onSeeAll={() => navigate("/favorites")}
        empty="Tap the heart on any track to favorite it."
        tracks={favorites}
        onPlay={(i) => playFromList(favorites, i)}
      />
    </div>
  );
}

function Section({
  title,
  icon,
  tracks,
  empty,
  onSeeAll,
  onPlay,
}: {
  title: string;
  icon: React.ReactNode;
  tracks: Track[];
  empty: string;
  onSeeAll: () => void;
  onPlay: (index: number) => void;
}) {
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--text-dim)" }}>
          {icon}
          <h2 className="disp" style={{ margin: 0, fontSize: 16, fontWeight: 600, color: "var(--text)" }}>
            {title}
          </h2>
        </div>
        {tracks.length > 0 && (
          <button className="btn" onClick={onSeeAll} style={{ padding: "6px 12px", fontSize: 12 }}>
            See all
          </button>
        )}
      </div>
      {tracks.length === 0 ? (
        <div className="empty-state" style={{ padding: "24px 0", textAlign: "left" }}>
          {empty}
        </div>
      ) : (
        <div style={{ maxWidth: 720 }}>
          {tracks.map((track, i) => (
            <TrackRow key={track.id} track={track} onPlay={() => onPlay(i)} />
          ))}
        </div>
      )}
    </div>
  );
}

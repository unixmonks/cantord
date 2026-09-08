import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import { IconGenre } from "../components/Icons";

export function GenresPage() {
  const { api, libraryVersion } = usePlayer();
  const navigate = useNavigate();
  const [genres, setGenres] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api
      .listGenres()
      .then(setGenres)
      .finally(() => setLoading(false));
  }, [api, libraryVersion]);

  if (loading) return <div className="empty-state">Loading genres…</div>;

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          Genres
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>{genres.length} genres</div>
      </div>

      {genres.length === 0 ? (
        <div className="empty-state">No genre tags found in the library.</div>
      ) : (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 12 }}>
          {genres.map((genre) => (
            <button
              key={genre}
              className="btn"
              style={{ padding: "12px 18px", fontSize: 14 }}
              onClick={() => navigate(`/genres/${encodeURIComponent(genre)}`)}
            >
              <IconGenre size={15} />
              {genre}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";

export function ArtistsPage() {
  const { api, libraryVersion } = usePlayer();
  const navigate = useNavigate();
  const [artists, setArtists] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api
      .listArtists()
      .then(setArtists)
      .finally(() => setLoading(false));
  }, [api, libraryVersion]);

  const half = Math.ceil(artists.length / 2);
  const left = artists.slice(0, half);
  const right = artists.slice(half);

  let lastLetterLeft = "";
  let lastLetterRight = "";

  if (loading) return <div className="empty-state">Loading artists…</div>;

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
          Artists
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>{artists.length} artists</div>
      </div>

      {artists.length === 0 ? (
        <div className="empty-state">No artists yet.</div>
      ) : (
        <div className="two-col-list" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 56px", maxWidth: 920 }}>
          <div>
            {left.map((name) => {
              const letter = name[0]?.toUpperCase() ?? "";
              const showLetter = letter !== lastLetterLeft;
              lastLetterLeft = letter;
              return (
                <ArtistEntry key={name} name={name} showLetter={showLetter} onClick={() => navigate(`/artists/${encodeURIComponent(name)}`)} />
              );
            })}
          </div>
          <div>
            {right.map((name) => {
              const letter = name[0]?.toUpperCase() ?? "";
              const showLetter = letter !== lastLetterRight;
              lastLetterRight = letter;
              return (
                <ArtistEntry key={name} name={name} showLetter={showLetter} onClick={() => navigate(`/artists/${encodeURIComponent(name)}`)} />
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function ArtistEntry({ name, showLetter, onClick }: { name: string; showLetter: boolean; onClick: () => void }) {
  return (
    <>
      {showLetter && (
        <div style={{ fontSize: 12, fontWeight: 700, color: "var(--accent)", padding: "14px 14px 6px" }}>{name[0]?.toUpperCase()}</div>
      )}
      <div
        className="row-hover"
        onClick={onClick}
        style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 14px", cursor: "pointer" }}
      >
        <span style={{ fontSize: 15, fontWeight: 500 }}>{name}</span>
      </div>
    </>
  );
}

import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { SearchResult } from "../api/types";
import { AlbumArt } from "../components/AlbumArt";
import { TrackRow } from "../components/TrackRow";
import { IconClose, IconSearch } from "../components/Icons";

export function SearchPage() {
  const { api, playFromList, artUpdate } = usePlayer();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const [query, setQuery] = useState(params.get("q") ?? "");
  const [result, setResult] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const q = params.get("q") ?? "";
    setQuery(q);
    if (!q.trim()) {
      setResult(null);
      return;
    }
    setLoading(true);
    const handle = setTimeout(() => {
      api
        .search(q, 25)
        .then(setResult)
        .finally(() => setLoading(false));
    }, 250);
    return () => clearTimeout(handle);
  }, [api, params]);

  useEffect(() => {
    if (!artUpdate) return;
    setResult((prev) =>
      prev && prev.albums.some((a) => a.id === artUpdate.albumId)
        ? { ...prev, albums: prev.albums.map((a) => (a.id === artUpdate.albumId ? { ...a, art_hash: artUpdate.artHash } : a)) }
        : prev,
    );
  }, [artUpdate]);

  function handleChange(value: string) {
    setQuery(value);
    if (value.trim()) setParams({ q: value }, { replace: true });
    else setParams({}, { replace: true });
  }

  return (
    <div>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 12,
          background: "var(--bg-elev)",
          border: "1px solid var(--border)",
          borderRadius: 10,
          padding: "12px 16px",
          maxWidth: 520,
          marginBottom: 32,
        }}
      >
        <IconSearch size={20} style={{ color: "var(--text-faint)", flexShrink: 0 }} />
        <input
          autoFocus
          value={query}
          onChange={(e) => handleChange(e.target.value)}
          placeholder="Search albums, artists, tracks…"
          style={{ flex: 1, background: "none", border: "none", outline: "none", color: "var(--text)", fontSize: 17 }}
        />
        {query && (
          <button className="iconbtn" onClick={() => handleChange("")} title="Clear">
            <IconClose size={16} />
          </button>
        )}
      </div>

      {!query.trim() && <div className="empty-state">Search your library by album, artist, or track title.</div>}
      {loading && <div className="empty-state">Searching…</div>}

      {result && !loading && (
        <>
          {result.artists.length === 0 && result.albums.length === 0 && result.tracks.length === 0 && (
            <div className="empty-state">No results for "{query}".</div>
          )}

          {result.artists.length > 0 && (
            <div style={{ marginBottom: 36 }}>
              <div className="section-label" style={{ marginBottom: 6 }}>
                Artists · {result.artists.length}
              </div>
              <div style={{ maxWidth: 640 }}>
                {result.artists.map((name) => (
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

          {result.albums.length > 0 && (
            <div style={{ marginBottom: 36 }}>
              <div className="section-label" style={{ marginBottom: 14 }}>
                Albums · {result.albums.length}
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 20, maxWidth: 900 }}>
                {result.albums.map((album) => (
                  <div
                    key={album.id}
                    className="row-hover"
                    onClick={() => navigate(`/albums/${album.id}`)}
                    style={{ display: "flex", alignItems: "center", gap: 14, padding: 10, cursor: "pointer" }}
                  >
                    <AlbumArt artHash={album.art_hash} seed={album.id} label={album.name} size={52} />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: 16, fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {album.name}
                      </div>
                      <div style={{ fontSize: 13, color: "var(--text-dim)" }}>
                        {album.album_artist}
                        {album.year ? ` · ${album.year}` : ""}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {result.tracks.length > 0 && (
            <div>
              <div className="section-label" style={{ marginBottom: 6 }}>
                Tracks · {result.tracks.length}
              </div>
              <div style={{ maxWidth: 640 }}>
                {result.tracks.map((track, i) => (
                  <TrackRow
                    key={track.id}
                    track={track}
                    onPlay={() => playFromList(result.tracks, i)}
                    onChange={(updated) =>
                      setResult((prev) =>
                        prev ? { ...prev, tracks: prev.tracks.map((t) => (t.id === updated.id ? updated : t)) } : prev,
                      )
                    }
                  />
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Album } from "../api/types";
import { AlbumArt } from "../components/AlbumArt";
import { IconBack } from "../components/Icons";
import { useBackNavigate } from "../hooks/useBackNavigate";

export function ArtistDetailPage() {
  const { name } = useParams<{ name: string }>();
  const { api, libraryVersion, artUpdate } = usePlayer();
  const navigate = useNavigate();
  const goBack = useBackNavigate("/artists");
  const [albums, setAlbums] = useState<Album[] | null>(null);

  useEffect(() => {
    if (!name) return;
    let cancelled = false;
    setAlbums(null);
    async function loadAll() {
      const collected: Album[] = [];
      let cursor: string | undefined;
      do {
        const page = await api.listAlbums(cursor, 100);
        collected.push(...page.albums.filter((a) => a.album_artist === name));
        cursor = page.next_cursor;
      } while (cursor);
      if (!cancelled) setAlbums(collected);
    }
    loadAll().catch(() => !cancelled && setAlbums([]));
    return () => {
      cancelled = true;
    };
  }, [api, name, libraryVersion]);

  useEffect(() => {
    if (!artUpdate) return;
    setAlbums((prev) =>
      prev && prev.some((a) => a.id === artUpdate.albumId)
        ? prev.map((a) => (a.id === artUpdate.albumId ? { ...a, art_hash: artUpdate.artHash } : a))
        : prev,
    );
  }, [artUpdate]);

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
          {name}
        </h1>
        <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>
          {albums ? `${albums.length} album${albums.length === 1 ? "" : "s"}` : "Loading…"}
        </div>
      </div>

      {albums && albums.length === 0 && <div className="empty-state">No albums found for this artist.</div>}

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))", gap: "28px 20px" }}>
        {albums?.map((album) => (
          <div key={album.id} onClick={() => navigate(`/albums/${album.id}`)} style={{ cursor: "pointer" }}>
            <AlbumArt artHash={album.art_hash} seed={album.id} label={album.name} size="fill" radius={8} />
            <div style={{ marginTop: 10 }}>
              <div className="disp" style={{ fontSize: 14, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {album.name}
              </div>
              <div style={{ fontSize: 11, color: "var(--text-faint)" }}>{album.year || ""}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

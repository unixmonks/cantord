import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import type { Album } from "../api/types";
import { AlbumArt } from "../components/AlbumArt";

// Persists loaded albums (and how far the user paginated) across mounts, so
// navigating to an album and back doesn't drop back to the first page and
// the top of the list. Invalidated whenever the library changes.
let albumsCache: {
  libraryVersion: number;
  albums: Album[];
  cursor: string | undefined;
  hasMore: boolean;
  index: Record<string, string>;
  activeLetter: string | null;
} | null = null;

export function AlbumsPage() {
  const { api, libraryVersion, artUpdate } = usePlayer();
  const navigate = useNavigate();
  const cached = albumsCache?.libraryVersion === libraryVersion ? albumsCache : null;
  const [albums, setAlbums] = useState<Album[]>(cached?.albums ?? []);
  const [cursor, setCursor] = useState<string | undefined>(cached?.cursor);
  const [hasMore, setHasMore] = useState(cached?.hasMore ?? true);
  const [loading, setLoading] = useState(false);
  const [index, setIndex] = useState<Record<string, string>>(cached?.index ?? {});
  const [activeLetter, setActiveLetter] = useState<string | null>(cached?.activeLetter ?? null);
  const loadedOnce = useRef(cached != null);
  const loadedVersion = useRef(cached?.libraryVersion);

  const loadPage = useCallback(
    async (after?: string, replace = false) => {
      setLoading(true);
      try {
        const page = await api.listAlbums(after, 30);
        setAlbums((prev) => (replace ? page.albums : [...prev, ...page.albums]));
        setCursor(page.next_cursor);
        setHasMore(Boolean(page.next_cursor));
      } finally {
        setLoading(false);
      }
    },
    [api],
  );

  useEffect(() => {
    if (loadedOnce.current && loadedVersion.current === libraryVersion) return;
    loadedOnce.current = true;
    loadedVersion.current = libraryVersion;
    setActiveLetter(null);
    loadPage(undefined, true);
    api.albumIndex().then(setIndex).catch(() => setIndex({}));
  }, [api, libraryVersion, loadPage]);

  useEffect(() => {
    albumsCache = { libraryVersion, albums, cursor, hasMore, index, activeLetter };
  }, [libraryVersion, albums, cursor, hasMore, index, activeLetter]);

  // Art enrichment arrives one album at a time, in the background, long
  // after the list was loaded — patch it in in place rather than reloading.
  useEffect(() => {
    if (!artUpdate) return;
    setAlbums((prev) =>
      prev.some((a) => a.id === artUpdate.albumId)
        ? prev.map((a) => (a.id === artUpdate.albumId ? { ...a, art_hash: artUpdate.artHash } : a))
        : prev,
    );
  }, [artUpdate]);

  function jumpTo(letter: string, letterCursor: string) {
    setActiveLetter(letter);
    setAlbums([]);
    loadPage(letterCursor, true);
  }

  return (
    <div style={{ display: "flex", gap: 8 }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ marginBottom: 24 }}>
          <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
            Albums
          </h1>
          <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>
            {albums.length} loaded{hasMore ? "" : " · all albums"}
          </div>
        </div>

        {albums.length === 0 && !loading && <div className="empty-state">No albums in the library yet.</div>}

        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))", gap: "28px 20px" }}>
          {albums.map((album) => (
            <div
              key={album.id}
              onClick={() => navigate(`/albums/${album.id}`)}
              style={{ cursor: "pointer" }}
            >
              <AlbumArt artHash={album.art_hash} seed={album.id} label={album.name} size="fill" radius={8} />
              <div style={{ marginTop: 10 }}>
                <div className="disp" style={{ fontSize: 14, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {album.name}
                </div>
                <div style={{ fontSize: 12, color: "var(--text-dim)", marginTop: 2, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {album.album_artist}
                </div>
                <div style={{ fontSize: 11, color: "var(--text-faint)" }}>{album.year || ""}</div>
              </div>
            </div>
          ))}
        </div>

        {hasMore && (
          <div style={{ display: "flex", justifyContent: "center", marginTop: 36 }}>
            <button className="btn" disabled={loading} onClick={() => loadPage(cursor)}>
              {loading ? "Loading…" : "Load more"}
            </button>
          </div>
        )}
      </div>

      {Object.keys(index).length > 0 && (
        <div style={{ width: 28, flexShrink: 0, display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 64, gap: 1 }}>
          {Object.keys(index)
            .sort()
            .map((letter) => (
              <button
                key={letter}
                onClick={() => jumpTo(letter, index[letter])}
                style={{
                  background: "none",
                  border: "none",
                  cursor: "pointer",
                  fontSize: 11,
                  fontWeight: 700,
                  padding: "3px 0",
                  width: 24,
                  color: activeLetter === letter ? "var(--accent)" : "var(--text-faint)",
                }}
              >
                {letter}
              </button>
            ))}
        </div>
      )}
    </div>
  );
}

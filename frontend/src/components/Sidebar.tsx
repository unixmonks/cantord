import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  IconAlbum,
  IconArtist,
  IconGenre,
  IconHome,
  IconPlaylist,
  IconQueue,
  IconSearch,
  IconSettings,
  IconWifiOff,
} from "./Icons";
import { usePlayer } from "../state/PlayerContext";

const NAV_ITEMS = [
  { root: "/", label: "Home", icon: IconHome, exact: true },
  { root: "/queue", label: "Queue", icon: IconQueue, exact: true },
  { root: "/albums", label: "Albums", icon: IconAlbum },
  { root: "/artists", label: "Artists", icon: IconArtist },
  { root: "/genres", label: "Genres", icon: IconGenre },
  { root: "/search", label: "Search", icon: IconSearch },
  { root: "/playlists", label: "Playlists", icon: IconPlaylist },
  { root: "/settings", label: "Settings", icon: IconSettings, exact: true },
];

// Sections with their own drill-down pages (an album, an artist, a
// playlist…). Clicking the sidebar item for one of these should take you
// back to the exact page you were last on in that section — e.g. the album
// you had open — not reset you to the section's list.
const RESUMABLE_ROOTS = ["/albums", "/artists", "/genres", "/playlists", "/search"];

export function Sidebar() {
  const { connected } = usePlayer();
  const location = useLocation();
  const [lastPaths, setLastPaths] = useState<Record<string, string>>({});

  useEffect(() => {
    const current = location.pathname + location.search;
    const root = RESUMABLE_ROOTS.find((r) => location.pathname === r || location.pathname.startsWith(`${r}/`));
    if (!root) return;
    setLastPaths((prev) => (prev[root] === current ? prev : { ...prev, [root]: current }));
  }, [location]);

  return (
    <aside
      style={{
        width: 240,
        flexShrink: 0,
        background: "var(--bg-elev)",
        borderRight: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        padding: "24px 16px",
      }}
    >
      <div className="disp" style={{ fontSize: 20, fontWeight: 700, letterSpacing: "0.01em", padding: "4px 12px 28px" }}>
        cantord
      </div>
      <nav style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {NAV_ITEMS.map(({ root, label, icon: Icon, exact }) => {
          const isActive = exact ? location.pathname === root : location.pathname === root || location.pathname.startsWith(`${root}/`);
          return (
            <Link
              key={root}
              to={lastPaths[root] ?? root}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                padding: "10px 12px",
                borderRadius: 8,
                background: isActive ? "var(--bg-elev-2)" : "transparent",
                color: isActive ? "var(--text)" : "var(--text-dim)",
              }}
            >
              <Icon size={18} />
              <span style={{ fontSize: 14, fontWeight: 600 }}>{label}</span>
            </Link>
          );
        })}
      </nav>
      <div style={{ flex: 1 }} />
      {!connected && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            padding: "10px 12px",
            fontSize: 12,
            color: "var(--danger)",
          }}
          title="Not connected to cantord"
        >
          <IconWifiOff size={14} />
          Disconnected
        </div>
      )}
    </aside>
  );
}

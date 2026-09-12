import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  IconAlbum,
  IconArtist,
  IconChevronLeft,
  IconClose,
  IconGenre,
  IconHeart,
  IconHome,
  IconPlaylist,
  IconQueue,
  IconSearch,
  IconSettings,
  IconSparkle,
  IconWifiOff,
} from "./Icons";
import { usePlayer } from "../state/PlayerContext";
import { useMediaQuery } from "../hooks/useMediaQuery";

const NAV_ITEMS = [
  { root: "/", label: "Home", icon: IconHome, exact: true },
  { root: "/queue", label: "Queue", icon: IconQueue, exact: true },
  { root: "/favorites", label: "Favorites", icon: IconHeart, exact: true },
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

const COLLAPSED_KEY = "sidebarCollapsed";

interface SidebarProps {
  mobileOpen?: boolean;
  onCloseMobile?: () => void;
}

export function Sidebar({ mobileOpen = false, onCloseMobile }: SidebarProps) {
  const { connected, aiPanelOpen, toggleAiPanel } = usePlayer();
  const location = useLocation();
  const isMobile = useMediaQuery("(max-width: 860px)");
  const [lastPaths, setLastPaths] = useState<Record<string, string>>({});
  const [collapsedPref, setCollapsedPref] = useState(() => {
    try {
      return localStorage.getItem(COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });
  // On mobile the sidebar is a full-width overlay drawer, not an icon rail —
  // the desktop collapse preference doesn't apply there.
  const collapsed = collapsedPref && !isMobile;

  useEffect(() => {
    const current = location.pathname + location.search;
    const root = RESUMABLE_ROOTS.find((r) => location.pathname === r || location.pathname.startsWith(`${r}/`));
    if (!root) return;
    setLastPaths((prev) => (prev[root] === current ? prev : { ...prev, [root]: current }));
  }, [location]);

  function toggleCollapsed() {
    setCollapsedPref((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        // ignore
      }
      return next;
    });
  }

  function navigateAway() {
    if (isMobile) onCloseMobile?.();
  }

  return (
    <aside
      className={`app-sidebar${mobileOpen ? " open" : ""}`}
      style={{
        width: collapsed ? 68 : 240,
        flexShrink: 0,
        background: "var(--bg-elev)",
        borderRight: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        padding: collapsed ? "24px 10px" : "24px 16px",
        transition: "width 0.15s ease, padding 0.15s ease",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: collapsed ? "center" : "space-between",
          padding: "4px 2px 28px",
          gap: 8,
        }}
      >
        {!collapsed && (
          <div className="disp" style={{ fontSize: 22, fontWeight: 700, letterSpacing: "0.01em", padding: "0 10px", whiteSpace: "nowrap" }}>
            cantord
          </div>
        )}
        <button
          className="iconbtn"
          onClick={isMobile ? onCloseMobile : toggleCollapsed}
          title={isMobile ? "Close menu" : collapsed ? "Expand sidebar" : "Collapse sidebar"}
          style={{ flexShrink: 0 }}
        >
          {isMobile ? (
            <IconClose size={18} />
          ) : (
            <IconChevronLeft size={18} style={{ transform: collapsed ? "rotate(180deg)" : "none", transition: "transform 0.15s ease" }} />
          )}
        </button>
      </div>
      <nav style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {NAV_ITEMS.map(({ root, label, icon: Icon, exact }) => {
          const isActive = exact ? location.pathname === root : location.pathname === root || location.pathname.startsWith(`${root}/`);
          return (
            <Link
              key={root}
              to={lastPaths[root] ?? root}
              title={collapsed ? label : undefined}
              onClick={navigateAway}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: collapsed ? "center" : "flex-start",
                gap: 12,
                padding: collapsed ? "10px" : "10px 12px",
                borderRadius: 8,
                background: isActive ? "var(--bg-elev-2)" : "transparent",
                color: isActive ? "var(--text)" : "var(--text-dim)",
              }}
            >
              <Icon size={20} style={{ flexShrink: 0 }} />
              {!collapsed && <span style={{ fontSize: 16, fontWeight: 600, whiteSpace: "nowrap" }}>{label}</span>}
            </Link>
          );
        })}
        <button
          onClick={() => {
            toggleAiPanel();
            navigateAway();
          }}
          title={collapsed ? "Assistant" : undefined}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: collapsed ? "center" : "flex-start",
            gap: 12,
            padding: collapsed ? "10px" : "10px 12px",
            borderRadius: 8,
            background: aiPanelOpen ? "var(--bg-elev-2)" : "transparent",
            color: aiPanelOpen ? "var(--text)" : "var(--text-dim)",
            border: "none",
            width: "100%",
            cursor: "pointer",
            font: "inherit",
          }}
        >
          <IconSparkle size={20} style={{ flexShrink: 0 }} />
          {!collapsed && <span style={{ fontSize: 16, fontWeight: 600, whiteSpace: "nowrap" }}>Assistant</span>}
        </button>
      </nav>
      <div style={{ flex: 1 }} />
      {!connected && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: collapsed ? "center" : "flex-start",
            gap: 8,
            padding: collapsed ? "10px" : "10px 12px",
            fontSize: 13,
            color: "var(--danger)",
          }}
          title="Not connected to cantord"
        >
          <IconWifiOff size={16} style={{ flexShrink: 0 }} />
          {!collapsed && <span style={{ whiteSpace: "nowrap" }}>Disconnected</span>}
        </div>
      )}
    </aside>
  );
}

import { useEffect, useRef, useState } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { PlayerBar } from "./PlayerBar";
import { Toast } from "./Toast";
import { AiPanel } from "./AiPanel";
import { IconMenu } from "./Icons";

// Keyed by route (pathname + search), not history entry, so returning to a
// page restores its scroll position no matter how you got there — browser
// back/forward, an in-app "Back" link, or just clicking the same sidebar
// item again after visiting somewhere else.
const scrollPositions = new Map<string, number>();

export function Layout() {
  const mainRef = useRef<HTMLElement>(null);
  const location = useLocation();
  const routeKey = location.pathname + location.search;
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  useEffect(() => {
    setMobileNavOpen(false);
  }, [routeKey]);

  useEffect(() => {
    const el = mainRef.current;
    if (!el) return;

    const target = scrollPositions.get(routeKey) ?? 0;
    el.scrollTop = target;

    // Content (album grids, track lists, …) often loads asynchronously after
    // mount, so the scrollable area may still be too short to reach the saved
    // offset on the first paint. Poll for a short window and re-apply once the
    // page has grown enough to hold it, without fighting a manual scroll.
    let cancelled = false;
    let attempts = 0;
    let raf = requestAnimationFrame(function retry() {
      if (cancelled || attempts++ > 30) return;
      if (el.scrollTop < target && el.scrollHeight - el.clientHeight >= target - 1) {
        el.scrollTop = target;
      }
      raf = requestAnimationFrame(retry);
    });

    const onScroll = () => scrollPositions.set(routeKey, el.scrollTop);
    el.addEventListener("scroll", onScroll, { passive: true });

    return () => {
      cancelled = true;
      cancelAnimationFrame(raf);
      el.removeEventListener("scroll", onScroll);
    };
  }, [routeKey]);

  return (
    <div className="app-shell">
      <div className="app-topbar">
        <button className="iconbtn" onClick={() => setMobileNavOpen(true)} title="Open menu">
          <IconMenu size={22} />
        </button>
        <span className="disp app-topbar-title">cantord</span>
      </div>
      <div className="app-body">
        <Sidebar mobileOpen={mobileNavOpen} onCloseMobile={() => setMobileNavOpen(false)} />
        {mobileNavOpen && <div className="sidebar-backdrop" onClick={() => setMobileNavOpen(false)} />}
        <main ref={mainRef} className="app-main">
          <Outlet />
        </main>
      </div>
      <PlayerBar />
      <Toast />
      <AiPanel />
    </div>
  );
}

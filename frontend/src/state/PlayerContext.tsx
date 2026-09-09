import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { ApiClient } from "../api/client";
import type { RepeatMode, ScanProgress, Status, Track } from "../api/types";

const STORAGE_KEY = "cantord:baseUrl";
const DEFAULT_BASE_URL = "http://localhost:8080";

function loadBaseUrl(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) || DEFAULT_BASE_URL;
  } catch {
    return DEFAULT_BASE_URL;
  }
}

interface ArtUpdate {
  albumId: string;
  artHash: string;
}

interface PlayerContextValue {
  api: ApiClient;
  baseUrl: string;
  setBaseUrl: (url: string) => void;
  connected: boolean;
  status: Status | null;
  queue: Track[];
  scanProgress: ScanProgress | null;
  libraryVersion: number;
  artUpdate: ArtUpdate | null;
  toast: string | null;
  showToast: (message: string) => void;
  aiPanelOpen: boolean;
  toggleAiPanel: () => void;
  refreshQueue: () => void;
  refreshStatus: () => void;
  play: () => void;
  pause: () => void;
  togglePlayPause: () => void;
  stop: () => void;
  next: () => void;
  previous: () => void;
  seek: (seconds: number) => void;
  setVolume: (volume: number) => void;
  toggleMute: () => void;
  toggleShuffle: () => void;
  cycleRepeat: () => void;
  enqueue: (trackId: string) => Promise<void>;
  playNext: (trackId: string) => Promise<void>;
  playFromList: (tracks: Track[], startIndex: number) => Promise<void>;
  moveQueue: (from: number, to: number) => Promise<void>;
  clearQueue: () => Promise<void>;
  removeFromQueue: (index: number) => Promise<void>;
  playIndex: (index: number) => Promise<void>;
}

const PlayerContext = createContext<PlayerContextValue | null>(null);

export function PlayerProvider({ children }: { children: ReactNode }) {
  const [baseUrl, setBaseUrlState] = useState(loadBaseUrl);
  const api = useMemo(() => new ApiClient(baseUrl), [baseUrl]);

  const [connected, setConnected] = useState(false);
  const [status, setStatus] = useState<Status | null>(null);
  const [queue, setQueue] = useState<Track[]>([]);
  const [scanProgress, setScanProgress] = useState<ScanProgress | null>(null);
  const [libraryVersion, setLibraryVersion] = useState(0);
  const [artUpdate, setArtUpdate] = useState<ArtUpdate | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const toggleAiPanel = useCallback(() => setAiPanelOpen((v) => !v), []);

  const showToast = useCallback((message: string) => {
    setToast(message);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 3500);
  }, []);

  const guard = useCallback(
    async (fn: () => Promise<unknown>) => {
      try {
        await fn();
      } catch (err) {
        showToast(err instanceof Error ? err.message : "Something went wrong");
      }
    },
    [showToast],
  );

  const setBaseUrl = useCallback((url: string) => {
    const trimmed = url.replace(/\/+$/, "");
    try {
      localStorage.setItem(STORAGE_KEY, trimmed);
    } catch {
      // ignore storage failures (private browsing, etc.)
    }
    setBaseUrlState(trimmed);
  }, []);

  const refreshQueue = useCallback(() => {
    api.getQueue().then(setQueue).catch(() => undefined);
  }, [api]);

  const refreshStatus = useCallback(() => {
    api.getStatus().then(setStatus).catch(() => undefined);
  }, [api]);

  // Initial load + SSE subscription. Re-runs whenever the base URL changes.
  useEffect(() => {
    let cancelled = false;
    setConnected(false);
    api.getStatus().then((s) => !cancelled && setStatus(s)).catch(() => undefined);
    api.getQueue().then((q) => !cancelled && setQueue(q)).catch(() => undefined);
    api.scanStatus().then((p) => !cancelled && setScanProgress(p)).catch(() => undefined);

    const source = new EventSource(api.eventsUrl());
    source.addEventListener("open", () => !cancelled && setConnected(true));
    source.addEventListener("error", () => !cancelled && setConnected(false));
    source.addEventListener("status", (e) => {
      if (cancelled) return;
      try {
        setStatus(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore malformed event */
      }
    });
    source.addEventListener("queue_changed", (e) => {
      if (cancelled) return;
      try {
        setQueue(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore malformed event */
      }
    });
    source.addEventListener("scan_progress", (e) => {
      if (cancelled) return;
      try {
        setScanProgress(JSON.parse((e as MessageEvent).data));
      } catch {
        /* ignore malformed event */
      }
    });
    source.addEventListener("library_changed", () => {
      if (cancelled) return;
      setLibraryVersion((v) => v + 1);
    });
    // Art enrichment runs in the background per-album and can fire for a
    // long time after a scan. It's cosmetic (a thumbnail arriving), not a
    // structural change, so it must NOT bump libraryVersion — pages that key
    // cached lists/pagination on libraryVersion would otherwise get reset
    // out from under the user every time a single album's art shows up.
    source.addEventListener("art_updated", (e) => {
      if (cancelled) return;
      try {
        const data = JSON.parse((e as MessageEvent).data) as { album_id: string; art_hash: string };
        setArtUpdate({ albumId: data.album_id, artHash: data.art_hash });
      } catch {
        /* ignore malformed event */
      }
    });

    return () => {
      cancelled = true;
      source.close();
    };
  }, [api]);

  const value = useMemo<PlayerContextValue>(
    () => ({
      api,
      baseUrl,
      setBaseUrl,
      connected,
      status,
      queue,
      scanProgress,
      libraryVersion,
      artUpdate,
      toast,
      showToast,
      aiPanelOpen,
      toggleAiPanel,
      refreshQueue,
      refreshStatus,
      play: () => guard(() => api.play()),
      pause: () => guard(() => api.pause()),
      togglePlayPause: () =>
        guard(() => (status?.state === "playing" ? api.pause() : api.play())),
      stop: () => guard(() => api.stop()),
      next: () => guard(() => api.next()),
      previous: () => guard(() => api.previous()),
      seek: (seconds: number) => guard(() => api.seek(seconds)),
      setVolume: (volume: number) => guard(() => api.setVolume(volume)),
      toggleMute: () => guard(() => api.setMute(!status?.muted)),
      toggleShuffle: () => guard(() => api.setShuffle(!status?.shuffle)),
      cycleRepeat: () => {
        const order: RepeatMode[] = ["off", "all", "one"];
        const current = status?.repeat ?? "off";
        const nextMode = order[(order.indexOf(current) + 1) % order.length];
        return guard(() => api.setRepeat(nextMode));
      },
      enqueue: (trackId: string) =>
        guard(async () => {
          await api.enqueue(trackId);
          refreshQueue();
          showToast("Added to queue");
        }),
      playNext: (trackId: string) =>
        guard(async () => {
          await api.playNext(trackId);
          refreshQueue();
          showToast("Playing next");
        }),
      playFromList: (tracks: Track[], startIndex: number) =>
        guard(async () => {
          const toQueue = tracks.slice(startIndex);
          if (toQueue.length === 0) return;
          await api.clearQueue();
          for (const track of toQueue) {
            await api.enqueue(track.id);
          }
          await api.playIndex(0);
          refreshQueue();
          refreshStatus();
        }),
      moveQueue: (from: number, to: number) =>
        guard(async () => {
          await api.moveQueue(from, to);
          refreshQueue();
        }),
      clearQueue: () =>
        guard(async () => {
          await api.clearQueue();
          refreshQueue();
        }),
      removeFromQueue: (index: number) =>
        guard(async () => {
          await api.removeFromQueue(index);
          refreshQueue();
        }),
      playIndex: (index: number) =>
        guard(async () => {
          await api.playIndex(index);
          refreshStatus();
        }),
    }),
    [
      api,
      baseUrl,
      setBaseUrl,
      connected,
      status,
      queue,
      scanProgress,
      libraryVersion,
      artUpdate,
      toast,
      showToast,
      aiPanelOpen,
      toggleAiPanel,
      refreshQueue,
      refreshStatus,
      guard,
    ],
  );

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>;
}

export function usePlayer(): PlayerContextValue {
  const ctx = useContext(PlayerContext);
  if (!ctx) throw new Error("usePlayer must be used within a PlayerProvider");
  return ctx;
}

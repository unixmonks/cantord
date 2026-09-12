import { useEffect, useState } from "react";
import { usePlayer } from "../state/PlayerContext";
import type { Stats } from "../api/types";
import { formatBytes, formatDurationLong } from "../utils/format";
import { IconRefresh } from "../components/Icons";

export function SettingsPage() {
  const { api, baseUrl, setBaseUrl, connected, scanProgress, showToast, libraryVersion } = usePlayer();
  const [addressDraft, setAddressDraft] = useState(baseUrl);
  const [version, setVersion] = useState<string | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);
  const [scanning, setScanning] = useState(false);

  useEffect(() => setAddressDraft(baseUrl), [baseUrl]);

  useEffect(() => {
    api.version().then((v) => setVersion(v.version)).catch(() => setVersion(null));
    api.libraryStats().then(setStats).catch(() => setStats(null));
  }, [api, connected, libraryVersion]);

  function commitAddress() {
    const next = addressDraft.trim();
    if (next && next !== baseUrl) setBaseUrl(next);
  }

  async function scanNow() {
    setScanning(true);
    try {
      await api.triggerScan();
      showToast("Library scan started");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't start scan");
    } finally {
      setScanning(false);
    }
  }

  const pct = scanProgress && scanProgress.total > 0 ? Math.round((scanProgress.processed / scanProgress.total) * 100) : 0;

  return (
    <div>
      <div style={{ marginBottom: 28 }}>
        <h1 className="disp" style={{ margin: 0, fontSize: 32, fontWeight: 700 }}>
          Settings
        </h1>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 24, maxWidth: 760 }}>
        <div className="card">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 18, gap: 12, flexWrap: "wrap" }}>
            <h2 className="disp" style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>
              Connection
            </h2>
            <div
              style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, color: connected ? "var(--good)" : "var(--danger)" }}
            >
              <div style={{ width: 7, height: 7, borderRadius: "50%", background: connected ? "var(--good)" : "var(--danger)" }} />
              {connected ? "Connected" : "Disconnected"}
              {version && connected ? ` · v${version}` : ""}
            </div>
          </div>
          <label style={{ fontSize: 13, color: "var(--text-faint)", display: "block", marginBottom: 6 }}>
            Server address (CANTORD_ADDR)
          </label>
          <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
            <input
              className="field"
              style={{ flex: "1 1 220px" }}
              value={addressDraft}
              onChange={(e) => setAddressDraft(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && commitAddress()}
              placeholder="http://localhost:8080"
            />
            <button className="btn btn-primary" onClick={commitAddress} disabled={!addressDraft.trim() || addressDraft.trim() === baseUrl}>
              Save
            </button>
          </div>
        </div>

        <div className="card">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 18, gap: 12, flexWrap: "wrap" }}>
            <h2 className="disp" style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>
              Library scan
            </h2>
            <button className="btn btn-primary" disabled={scanning || scanProgress?.running} onClick={scanNow}>
              <IconRefresh size={15} /> Scan library
            </button>
          </div>

          {scanProgress?.running ? (
            <>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
                <span style={{ fontSize: 14, color: "var(--text-dim)" }}>
                  Scanning — {scanProgress.processed.toLocaleString()} / {scanProgress.total.toLocaleString()}
                </span>
                <span style={{ fontSize: 14, color: "var(--text-dim)" }}>{pct}%</span>
              </div>
              <div style={{ height: 6, background: "var(--bg-elev-2)", borderRadius: 3, overflow: "hidden", marginBottom: 12 }}>
                <div style={{ height: "100%", width: `${pct}%`, background: "var(--accent)", borderRadius: 3 }} />
              </div>
              {scanProgress.current_path && (
                <div
                  style={{
                    fontSize: 13,
                    color: "var(--text-faint)",
                    fontFamily: "monospace",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    marginBottom: 24,
                  }}
                >
                  {scanProgress.current_path}
                </div>
              )}
            </>
          ) : (
            <div style={{ fontSize: 14, color: "var(--text-faint)", marginBottom: 24 }}>
              {scanProgress ? "Idle — library is up to date." : "Scan status unavailable."}
            </div>
          )}

          {scanProgress && (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(90px, 1fr))", gap: 20, paddingTop: 20, borderTop: "1px solid var(--border)" }}>
              <Stat label="Added / updated" value={scanProgress.added_or_updated} color="var(--accent)" />
              <Stat label="Unchanged" value={scanProgress.skipped_unchanged} />
              <Stat label="Marked missing" value={scanProgress.marked_missing} />
              <Stat label="Marked available" value={scanProgress.marked_available} />
              <Stat label="Failed" value={scanProgress.failed} color={scanProgress.failed > 0 ? "var(--danger)" : undefined} />
              <Stat label="Total" value={scanProgress.total} />
            </div>
          )}
        </div>

        {stats && (
          <div className="card">
            <h2 className="disp" style={{ margin: "0 0 18px", fontSize: 18, fontWeight: 600 }}>
              Library
            </h2>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(90px, 1fr))", gap: 20 }}>
              <Stat label="Tracks" value={stats.tracks} />
              <Stat label="Albums" value={stats.albums} />
              <Stat label="Artists" value={stats.artists} />
              <Stat label="Unavailable" value={stats.unavailable} color={stats.unavailable > 0 ? "var(--danger)" : undefined} />
            </div>
            <div style={{ display: "flex", gap: 24, marginTop: 20, paddingTop: 20, borderTop: "1px solid var(--border)", fontSize: 14, color: "var(--text-dim)", flexWrap: "wrap" }}>
              <span>{formatBytes(stats.total_size_bytes)} on disk</span>
              <span>{formatDurationLong(stats.total_duration_ms)} of music</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Stat({ label, value, color }: { label: string; value: number; color?: string }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <div className="disp" style={{ fontSize: 24, fontWeight: 600, color: color ?? "var(--text)" }}>
        {(value ?? 0).toLocaleString()}
      </div>
      <div className="section-label">{label}</div>
    </div>
  );
}

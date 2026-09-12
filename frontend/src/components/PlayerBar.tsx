import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { usePlayer } from "../state/PlayerContext";
import { AlbumArt } from "./AlbumArt";
import { Slider } from "./Slider";
import { formatDuration } from "../utils/format";
import { useMediaQuery } from "../hooks/useMediaQuery";
import {
  IconHeart,
  IconHeartFilled,
  IconNext,
  IconPause,
  IconPlay,
  IconPrev,
  IconRepeat,
  IconRepeatOne,
  IconShuffle,
  IconStop,
  IconVolume,
  IconVolumeMuted,
} from "./Icons";

export function PlayerBar() {
  const { api, status, showToast, togglePlayPause, stop, next, previous, seek, setVolume, toggleMute, toggleShuffle, cycleRepeat } =
    usePlayer();
  const navigate = useNavigate();
  const isNarrow = useMediaQuery("(max-width: 720px)");
  const isTiny = useMediaQuery("(max-width: 480px)");

  const track = status?.track;
  const duration = status?.duration_ms ?? 0;
  const [localPos, setLocalPos] = useState(status?.position_ms ?? 0);

  useEffect(() => {
    setLocalPos(status?.position_ms ?? 0);
  }, [status?.position_ms, status?.track?.id]);

  // The engine's status snapshot doesn't get a fresh `favorite` flag pushed
  // to it when it changes elsewhere, so track our own optimistic override
  // for the currently playing track rather than relying on status.track.
  const [favOverride, setFavOverride] = useState<{ id: string; value: boolean } | null>(null);
  useEffect(() => {
    setFavOverride(null);
  }, [track?.id]);
  const isFavorite = track ? (favOverride?.id === track.id ? favOverride.value : track.favorite) : false;

  async function toggleFavorite() {
    if (!track) return;
    const nextValue = !isFavorite;
    setFavOverride({ id: track.id, value: nextValue });
    try {
      await api.setFavorite(track.id, nextValue);
    } catch (err) {
      setFavOverride({ id: track.id, value: !nextValue });
      showToast(err instanceof Error ? err.message : "Couldn't update favorite");
    }
  }

  useEffect(() => {
    if (status?.state !== "playing" || !duration) return;
    const id = setInterval(() => {
      setLocalPos((p) => Math.min(p + 1000, duration));
    }, 1000);
    return () => clearInterval(id);
  }, [status?.state, duration]);

  const [dragPos, setDragPos] = useState<number | null>(null);
  const [dragVol, setDragVol] = useState<number | null>(null);

  // Once the user releases the slider, keep showing their chosen value
  // (rather than falling back to the last known server value) until the
  // backend confirms it via a status event. Otherwise the slider snaps
  // back to the stale value for the round-trip and then jumps forward.
  useEffect(() => {
    if (dragVol !== null && status?.volume === dragVol) {
      setDragVol(null);
    }
  }, [status?.volume, dragVol]);

  const progressRatio = duration > 0 ? (dragPos ?? localPos) / duration : 0;
  const volumeRatio = (dragVol ?? status?.volume ?? 0) / 100;

  if (!track) return null;

  return (
    <footer
      className="player-bar"
      style={{
        height: 88,
        flexShrink: 0,
        background: "var(--bg-elev)",
        borderTop: "1px solid var(--border)",
        display: "flex",
        alignItems: "center",
        padding: isTiny ? "0 12px" : isNarrow ? "0 16px" : "0 24px",
        gap: isNarrow ? 12 : 24,
      }}
    >
      <div
        className="player-track-info"
        style={{
          display: "flex",
          alignItems: "center",
          gap: isTiny ? 8 : 12,
          width: isNarrow ? "auto" : 280,
          maxWidth: isTiny ? 130 : isNarrow ? 220 : undefined,
          flexShrink: 0,
          minWidth: 0,
        }}
      >
        <AlbumArt
          artHash={track.art_hash}
          seed={track.album_id || track.album}
          label={track.album || track.title}
          size={isTiny ? 40 : 52}
        />
        <div style={{ minWidth: 0 }}>
          <div className="disp" style={{ fontSize: 16, fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {track.title}
          </div>
          {!isTiny && (
          <div style={{ fontSize: 13, color: "var(--text-dim)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {track.artist && (
              <span
                className="meta-link"
                style={{ cursor: "pointer" }}
                onClick={() => navigate(`/artists/${encodeURIComponent(track.album_artist || track.artist)}`)}
              >
                {track.artist}
              </span>
            )}
            {track.album && (
              <>
                {track.artist ? " — " : ""}
                <span
                  className="meta-link"
                  style={{ cursor: track.album_id ? "pointer" : "default" }}
                  onClick={() => track.album_id && navigate(`/albums/${track.album_id}`)}
                >
                  {track.album}
                </span>
              </>
            )}
          </div>
          )}
        </div>
      </div>

      {!isTiny && (
        <button
          className={`iconbtn${isFavorite ? " active" : ""}`}
          onClick={toggleFavorite}
          title={isFavorite ? "Remove from favorites" : "Add to favorites"}
        >
          {isFavorite ? <IconHeartFilled size={18} /> : <IconHeart size={18} />}
        </button>
      )}

      <div className="player-controls" style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: 6, maxWidth: 640, margin: "0 auto" }}>
        <div style={{ display: "flex", alignItems: "center", gap: isTiny ? 8 : 18 }}>
          {!isTiny && (
            <button
              className={`iconbtn${status?.shuffle ? " active" : ""}`}
              onClick={toggleShuffle}
              title="Shuffle remaining queue"
            >
              <IconShuffle size={18} />
            </button>
          )}
          <button className="iconbtn" onClick={previous} title="Previous">
            <IconPrev size={20} />
          </button>
          <button
            onClick={togglePlayPause}
            title={status?.state === "playing" ? "Pause" : "Play"}
            style={{
              width: 36,
              height: 36,
              borderRadius: "50%",
              background: "var(--text)",
              color: "var(--bg)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              border: "none",
              cursor: "pointer",
            }}
          >
            {status?.state === "playing" ? <IconPause size={17} /> : <IconPlay size={17} />}
          </button>
          <button className="iconbtn" onClick={next} title="Next">
            <IconNext size={20} />
          </button>
          {!isTiny && (
            <button className="iconbtn" onClick={stop} title="Stop">
              <IconStop size={17} />
            </button>
          )}
          <button
            className={`iconbtn${status?.repeat && status.repeat !== "off" ? " active" : ""}`}
            onClick={cycleRepeat}
            title={`Repeat: ${status?.repeat ?? "off"}`}
          >
            {status?.repeat === "one" ? <IconRepeatOne size={18} /> : <IconRepeat size={18} />}
          </button>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, width: "100%" }}>
          {!isTiny && (
            <span style={{ fontSize: 12, color: "var(--text-faint)", width: 32, textAlign: "right" }}>
              {formatDuration(dragPos !== null ? dragPos * duration : localPos)}
            </span>
          )}
          <Slider
            value={progressRatio}
            onDragValue={(r) => setDragPos(r)}
            onCommit={(r) => {
              setDragPos(null);
              if (duration > 0) seek((r * duration) / 1000);
            }}
          />
          {!isTiny && <span style={{ fontSize: 12, color: "var(--text-faint)", width: 32 }}>{formatDuration(duration)}</span>}
        </div>
      </div>

      <div
        className="player-volume"
        style={{ display: isNarrow ? "none" : "flex", alignItems: "center", gap: 10, width: 160, flexShrink: 0, justifyContent: "flex-end" }}
      >
        <button className="iconbtn" onClick={toggleMute} title={status?.muted ? "Unmute" : "Mute"}>
          {status?.muted ? <IconVolumeMuted size={19} /> : <IconVolume size={19} />}
        </button>
        <div style={{ width: 88 }}>
          <Slider
            value={volumeRatio}
            accent={status?.muted ? "var(--text-faint)" : "var(--accent)"}
            onDragValue={(r) => setDragVol(r * 100)}
            onCommit={(r) => {
              const v = Math.round(r * 100);
              setDragVol(v);
              setVolume(v);
            }}
          />
        </div>
      </div>
    </footer>
  );
}

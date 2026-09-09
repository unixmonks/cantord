import { useState, type DragEvent, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import type { Track } from "../api/types";
import { formatDuration } from "../utils/format";
import { usePlayer } from "../state/PlayerContext";
import { AlbumArt } from "./AlbumArt";
import { AddToPlaylistMenu } from "./AddToPlaylistMenu";
import {
  IconGrip,
  IconHeart,
  IconHeartFilled,
  IconPlus,
  IconStar,
  IconStarFilled,
  IconTrash,
} from "./Icons";

interface TrackRowProps {
  track: Track;
  index?: number;
  showArt?: boolean;
  showAlbum?: boolean;
  isPlaying?: boolean;
  isPaused?: boolean;
  onPlay?: () => void;
  onRemove?: () => void;
  onChange?: (track: Track) => void;
  showRating?: boolean;
  hideAdd?: boolean;
  extraActions?: ReactNode;
  draggable?: boolean;
  onDragStart?: (e: DragEvent) => void;
  onDragOver?: (e: DragEvent) => void;
  onDrop?: (e: DragEvent) => void;
  onDragEnd?: (e: DragEvent) => void;
  dragOver?: boolean;
}

export function TrackRow({
  track,
  index,
  showArt = true,
  showAlbum = true,
  isPlaying = false,
  isPaused = false,
  onPlay,
  onRemove,
  onChange,
  showRating = false,
  hideAdd = false,
  extraActions,
  draggable = false,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  dragOver = false,
}: TrackRowProps) {
  const { api, enqueue, showToast } = usePlayer();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);

  async function toggleFavorite() {
    setBusy(true);
    try {
      await api.setFavorite(track.id, !track.favorite);
      onChange?.({ ...track, favorite: !track.favorite });
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't update favorite");
    } finally {
      setBusy(false);
    }
  }

  async function rate(rating: number) {
    const next = rating === track.rating ? 0 : rating;
    setBusy(true);
    try {
      await api.setRating(track.id, next);
      onChange?.({ ...track, rating: next });
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Couldn't update rating");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="row-hover"
      draggable={draggable}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={onDragEnd}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 14,
        padding: "9px 10px",
        cursor: onPlay ? "pointer" : "default",
        background: isPlaying ? "var(--bg-elev-2)" : dragOver ? "var(--bg-elev-3)" : undefined,
      }}
      onClick={onPlay}
    >
      {draggable && (
        <span style={{ color: "var(--text-faint)", cursor: "grab" }}>
          <IconGrip size={14} />
        </span>
      )}
      {index !== undefined && (
        <div style={{ width: 20, textAlign: "center", fontSize: 13, color: isPlaying ? "var(--accent)" : "var(--text-faint)", flexShrink: 0 }}>
          {isPlaying ? <EqGlyph paused={isPaused} /> : index}
        </div>
      )}
      {showArt && <AlbumArt artHash={track.art_hash} seed={track.album_id || track.album} label={track.album || track.title} size={40} />}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontSize: 14,
            fontWeight: 600,
            color: isPlaying ? "var(--accent)" : "var(--text)",
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
        >
          {track.title || "Untitled"}
        </div>
        {showAlbum && (
          <div style={{ fontSize: 12, color: "var(--text-dim)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {track.artist && (
              <span
                className="meta-link"
                style={{ cursor: "pointer" }}
                onClick={(e) => {
                  e.stopPropagation();
                  navigate(`/artists/${encodeURIComponent(track.album_artist || track.artist)}`);
                }}
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
                  onClick={(e) => {
                    e.stopPropagation();
                    if (track.album_id) navigate(`/albums/${track.album_id}`);
                  }}
                >
                  {track.album}
                </span>
              </>
            )}
          </div>
        )}
      </div>

      {showRating && (
        <div
          style={{ display: "flex", gap: 1, flexShrink: 0 }}
          onClick={(e) => e.stopPropagation()}
        >
          {[1, 2, 3, 4, 5].map((n) => (
            <button
              key={n}
              className="iconbtn"
              style={{ width: 18, height: 18, color: n <= track.rating ? "var(--accent)" : "var(--text-faint)" }}
              disabled={busy}
              onClick={() => rate(n)}
              title={`Rate ${n}`}
            >
              {n <= track.rating ? <IconStarFilled size={12} /> : <IconStar size={12} />}
            </button>
          ))}
        </div>
      )}

      <div style={{ fontSize: 12, color: "var(--text-faint)", width: 44, textAlign: "right", flexShrink: 0 }}>
        {formatDuration(track.duration_ms)}
      </div>

      <div className="row-actions" style={{ display: "flex", alignItems: "center", gap: 2, flexShrink: 0 }} onClick={(e) => e.stopPropagation()}>
        <button
          className={`iconbtn${track.favorite ? " active" : ""}`}
          disabled={busy}
          onClick={toggleFavorite}
          title={track.favorite ? "Remove from favorites" : "Add to favorites"}
        >
          {track.favorite ? <IconHeartFilled size={15} /> : <IconHeart size={15} />}
        </button>
        {!hideAdd && (
          <button className="iconbtn" onClick={() => enqueue(track.id)} title="Add to queue">
            <IconPlus size={15} />
          </button>
        )}
        <AddToPlaylistMenu trackIds={[track.id]} />
        {extraActions}
        {onRemove && (
          <button className="iconbtn" onClick={onRemove} title="Remove">
            <IconTrash size={15} />
          </button>
        )}
      </div>
    </div>
  );
}

function EqGlyph({ paused = false }: { paused?: boolean }) {
  return (
    <span className={`eq-bars${paused ? " is-paused" : ""}`}>
      <span className="eq-bar" />
      <span className="eq-bar" />
      <span className="eq-bar" />
    </span>
  );
}

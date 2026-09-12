import { useState, type DragEvent } from "react";
import { usePlayer } from "../state/PlayerContext";
import { TrackRow } from "../components/TrackRow";
import { IconTrash } from "../components/Icons";
import { formatDurationLong } from "../utils/format";

export function QueuePage() {
  const { queue, status, playIndex, removeFromQueue, moveQueue, clearQueue, refreshQueue } = usePlayer();
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  const totalMs = queue.reduce((sum, t) => sum + t.duration_ms, 0);

  function onDragStart(index: number) {
    return (e: DragEvent) => {
      setDragIndex(index);
      e.dataTransfer.effectAllowed = "move";
    };
  }

  function onDragOver(index: number) {
    return (e: DragEvent) => {
      e.preventDefault();
      setOverIndex(index);
    };
  }

  function onDrop(index: number) {
    return (e: DragEvent) => {
      e.preventDefault();
      setOverIndex(null);
      if (dragIndex !== null && dragIndex !== index) {
        moveQueue(dragIndex, index);
      }
      setDragIndex(null);
    };
  }

  return (
    <div>
      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", marginBottom: 24, gap: 12, flexWrap: "wrap" }}>
        <div>
          <h1 className="disp" style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>
            Queue
          </h1>
          <div style={{ fontSize: 13, color: "var(--text-faint)", marginTop: 4 }}>
            {queue.length} track{queue.length === 1 ? "" : "s"}
            {totalMs ? ` · ${formatDurationLong(totalMs)}` : ""}
          </div>
        </div>
        <button className="btn" disabled={queue.length === 0} onClick={clearQueue}>
          <IconTrash size={14} /> Clear queue
        </button>
      </div>

      {queue.length === 0 ? (
        <div className="empty-state">Queue is empty. Add tracks from Albums, Search, or a playlist.</div>
      ) : (
        <>
          <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {queue.map((track, i) => (
              <TrackRow
                key={`${track.id}-${i}`}
                track={track}
                index={i + 1}
                isPlaying={status?.state !== "stopped" && status?.queue_index === i}
                isPaused={status?.state === "paused" && status?.queue_index === i}
                hideAdd
                onChange={() => refreshQueue()}
                onPlay={() => playIndex(i)}
                onRemove={() => removeFromQueue(i)}
                draggable
                dragOver={overIndex === i}
                onDragStart={onDragStart(i)}
                onDragOver={onDragOver(i)}
                onDrop={onDrop(i)}
                onDragEnd={() => {
                  setDragIndex(null);
                  setOverIndex(null);
                }}
              />
            ))}
          </div>
          <div style={{ marginTop: 28, padding: "16px 18px", border: "1px dashed var(--border)", borderRadius: 10, fontSize: 12, color: "var(--text-faint)" }}>
            Drag the grip to reorder · click a row to jump playback there · trash removes it from the queue
          </div>
        </>
      )}
    </div>
  );
}

import { useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

interface SliderProps {
  value: number; // 0..1, ignored while dragging (local value shown instead)
  onCommit: (value: number) => void;
  onDragValue?: (value: number) => void;
  disabled?: boolean;
  height?: number;
  accent?: string;
}

export function Slider({ value, onCommit, onDragValue, disabled, height = 4, accent = "var(--accent)" }: SliderProps) {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const [dragValue, setDragValue] = useState<number | null>(null);

  function ratioFromEvent(e: ReactPointerEvent<HTMLDivElement>): number {
    const el = trackRef.current;
    if (!el) return value;
    const rect = el.getBoundingClientRect();
    const ratio = (e.clientX - rect.left) / rect.width;
    return Math.min(1, Math.max(0, ratio));
  }

  function handlePointerDown(e: ReactPointerEvent<HTMLDivElement>) {
    if (disabled) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    const ratio = ratioFromEvent(e);
    setDragValue(ratio);
    onDragValue?.(ratio);
  }

  function handlePointerMove(e: ReactPointerEvent<HTMLDivElement>) {
    if (disabled || dragValue === null) return;
    const ratio = ratioFromEvent(e);
    setDragValue(ratio);
    onDragValue?.(ratio);
  }

  function handlePointerUp(e: ReactPointerEvent<HTMLDivElement>) {
    if (disabled || dragValue === null) return;
    const ratio = ratioFromEvent(e);
    setDragValue(null);
    onCommit(ratio);
  }

  const shown = dragValue !== null ? dragValue : value;

  return (
    <div
      ref={trackRef}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      style={{
        flex: 1,
        height: 14,
        display: "flex",
        alignItems: "center",
        cursor: disabled ? "default" : "pointer",
        touchAction: "none",
      }}
    >
      <div style={{ position: "relative", width: "100%", height, background: "var(--bg-elev-2)", borderRadius: height / 2 }}>
        <div
          style={{
            position: "absolute",
            left: 0,
            top: 0,
            height: "100%",
            width: `${shown * 100}%`,
            background: disabled ? "var(--text-faint)" : accent,
            borderRadius: height / 2,
          }}
        />
        <div
          style={{
            position: "absolute",
            left: `${shown * 100}%`,
            top: "50%",
            transform: "translate(-50%, -50%)",
            width: 10,
            height: 10,
            borderRadius: "50%",
            background: disabled ? "var(--text-faint)" : "var(--text)",
            opacity: dragValue !== null ? 1 : undefined,
          }}
        />
      </div>
    </div>
  );
}

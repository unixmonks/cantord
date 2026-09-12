import { usePlayer } from "../state/PlayerContext";

export function Toast() {
  const { toast } = usePlayer();
  if (!toast) return null;
  return (
    <div
      style={{
        position: "fixed",
        bottom: 104,
        left: "50%",
        transform: "translateX(-50%)",
        background: "var(--bg-elev-3)",
        border: "1px solid var(--border)",
        borderRadius: 10,
        padding: "10px 18px",
        fontSize: 14,
        fontWeight: 600,
        color: "var(--text)",
        boxShadow: "0 8px 24px rgba(0,0,0,0.35)",
        zIndex: 50,
      }}
    >
      {toast}
    </div>
  );
}

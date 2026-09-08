import { useState } from "react";
import { usePlayer } from "../state/PlayerContext";

const GRADIENTS: [string, string][] = [
  ["#3a2e6e", "#7b4fae"],
  ["#1f4d5c", "#2f9e8f"],
  ["#6b2d3c", "#c9584f"],
  ["#2d3a5c", "#4f7bc9"],
  ["#4a3b1f", "#c99a4f"],
  ["#2d5c3a", "#4fc978"],
  ["#5c2d54", "#c94fa8"],
  ["#3a3a3a", "#6b6b6b"],
];

function hash(input: string): number {
  let h = 0;
  for (let i = 0; i < input.length; i++) {
    h = (h * 31 + input.charCodeAt(i)) | 0;
  }
  return Math.abs(h);
}

interface AlbumArtProps {
  artHash?: string;
  seed: string;
  label: string;
  size: number | "fill";
  radius?: number;
}

export function AlbumArt({ artHash, seed, label, size, radius = 6 }: AlbumArtProps) {
  const { api } = usePlayer();
  const [failed, setFailed] = useState(false);
  const [g1, g2] = GRADIENTS[hash(seed) % GRADIENTS.length];
  const letter = (label.trim()[0] || "?").toUpperCase();
  const fill = size === "fill";
  const px = fill ? 0 : size;

  const style: React.CSSProperties = fill
    ? {
        width: "100%",
        aspectRatio: "1 / 1",
        borderRadius: radius,
        background: `linear-gradient(135deg, ${g1}, ${g2})`,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        overflow: "hidden",
        position: "relative",
      }
    : {
        width: px,
        height: px,
        borderRadius: radius,
        background: `linear-gradient(135deg, ${g1}, ${g2})`,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexShrink: 0,
        overflow: "hidden",
        position: "relative",
      };

  const fontSize = fill ? "40%" : px * 0.4;

  if (artHash && !failed) {
    return (
      <div style={style}>
        <img
          src={api.artUrl(artHash, fill || px > 64 ? "full" : "thumb")}
          alt=""
          onError={() => setFailed(true)}
          style={{ width: "100%", height: "100%", objectFit: "cover" }}
        />
      </div>
    );
  }

  return (
    <div style={style}>
      <span
        className="disp"
        style={{
          fontSize,
          fontWeight: 700,
          color: "rgba(255,255,255,0.35)",
        }}
      >
        {letter}
      </span>
    </div>
  );
}

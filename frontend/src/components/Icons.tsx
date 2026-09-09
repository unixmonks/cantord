import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement> & { size?: number };

function base({ size = 18, ...props }: IconProps) {
  return { width: size, height: size, viewBox: "0 0 20 20", ...props };
}

export function IconQueue(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M4 5h12M4 10h12M4 15h8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function IconAlbum(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.6" />
      <circle cx="10" cy="10" r="2" stroke="currentColor" strokeWidth="1.6" />
    </svg>
  );
}

export function IconArtist(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <circle cx="10" cy="6.5" r="3" stroke="currentColor" strokeWidth="1.6" />
      <path d="M4 17c0-3.3 2.7-5.5 6-5.5s6 2.2 6 5.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function IconGenre(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M4 4h5.5L16 10.5 10.5 16 4 9.5V4z" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
      <circle cx="7" cy="7" r="1" fill="currentColor" />
    </svg>
  );
}

export function IconSearch(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <circle cx="8.5" cy="8.5" r="5.5" stroke="currentColor" strokeWidth="1.6" />
      <path d="M16 16l-3.2-3.2" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function IconPlaylist(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M4 5h9M4 9.5h9M4 14h5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      <circle cx="15.5" cy="15" r="1.8" fill="currentColor" />
      <path d="M17.3 15V6l-2 0.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function IconHome(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M4 9.5L10 4l6 5.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5.5 8.5V16h9V8.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconSettings(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <circle cx="10" cy="10" r="2.6" stroke="currentColor" strokeWidth="1.6" />
      <path
        d="M10 3.5v2M10 14.5v2M16.5 10h-2M5.5 10h-2M14.6 5.4l-1.4 1.4M6.8 13.2l-1.4 1.4M14.6 14.6l-1.4-1.4M6.8 6.8L5.4 5.4"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function IconBack(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M12.5 4.5L7 10l5.5 5.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconTrash(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path
        d="M4.5 6h11M8 6V4.5h4V6M6 6l.6 9.5a1 1 0 0 0 1 .95h4.8a1 1 0 0 0 1-.95L14 6"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconPlus(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M10 4.5v11M4.5 10h11" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  );
}

export function IconClose(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function IconPlay(props: IconProps) {
  return (
    <svg {...base(props)}>
      <path d="M6 4l10 6-10 6V4z" fill="currentColor" />
    </svg>
  );
}

export function IconPause(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="5" y="4" width="3.5" height="12" rx="1" fill="currentColor" />
      <rect x="11.5" y="4" width="3.5" height="12" rx="1" fill="currentColor" />
    </svg>
  );
}

export function IconPrev(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="4" y="4" width="1.8" height="12" rx="0.9" fill="currentColor" />
      <path d="M16 5v10L7 10l9-5z" fill="currentColor" />
    </svg>
  );
}

export function IconNext(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="14.2" y="4" width="1.8" height="12" rx="0.9" fill="currentColor" />
      <path d="M4 5v10l9-5-9-5z" fill="currentColor" />
    </svg>
  );
}

export function IconStop(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="5" y="5" width="10" height="10" rx="1.5" fill="currentColor" />
    </svg>
  );
}

export function IconVolume(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M3 8v4h3l4.5 3.5v-11L6 8H3z" fill="currentColor" stroke="none" />
      <path d="M13.5 7.5a4 4 0 0 1 0 5M15.7 5.3a7.2 7.2 0 0 1 0 9.4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconVolumeMuted(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M3 8v4h3l4.5 3.5v-11L6 8H3z" fill="currentColor" stroke="none" />
      <path d="M14 8l4 4M18 8l-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconShuffle(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M3.5 6h2.8L13 14h3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M3.5 14h2.8L13 6h3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M14.5 3.8L16.5 6l-2 2.2M14.5 16.2L16.5 14l-2-2.2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconRepeat(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M5 7h8a2.5 2.5 0 0 1 2.5 2.5V11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M15 13H7a2.5 2.5 0 0 1-2.5-2.5V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M7 4.5L5 7l2 2.5M13 15.5l2-2.5-2-2.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconRepeatOne(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M5 7h8a2.5 2.5 0 0 1 2.5 2.5V11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M15 13H7a2.5 2.5 0 0 1-2.5-2.5V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M7 4.5L5 7l2 2.5M13 15.5l2-2.5-2-2.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <text x="10" y="11.6" textAnchor="middle" fontSize="6.5" fontWeight="700" fill="currentColor" stroke="none" fontFamily="sans-serif">
        1
      </text>
    </svg>
  );
}

export function IconHeart(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path
        d="M10 16.2s-6-3.7-6-8.1a3.6 3.6 0 0 1 6-2.7 3.6 3.6 0 0 1 6 2.7c0 4.4-6 8.1-6 8.1z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconHeartFilled(props: IconProps) {
  return (
    <svg {...base(props)}>
      <path d="M10 16.2s-6-3.7-6-8.1a3.6 3.6 0 0 1 6-2.7 3.6 3.6 0 0 1 6 2.7c0 4.4-6 8.1-6 8.1z" fill="currentColor" />
    </svg>
  );
}

export function IconStar(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path
        d="M10 3.6l1.85 3.75 4.15.6-3 2.93.7 4.12L10 13.05l-3.7 1.95.7-4.12-3-2.93 4.15-.6L10 3.6z"
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconStarFilled(props: IconProps) {
  return (
    <svg {...base(props)}>
      <path d="M10 3.6l1.85 3.75 4.15.6-3 2.93.7 4.12L10 13.05l-3.7 1.95.7-4.12-3-2.93 4.15-.6L10 3.6z" fill="currentColor" />
    </svg>
  );
}

export function IconClock(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 6v4.2l3 1.8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconRefresh(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M16 5.5a6.5 6.5 0 1 0 1.5 4.1" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      <path d="M16 3v3h-3" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconDots(props: IconProps) {
  return (
    <svg {...base(props)}>
      <circle cx="10" cy="4.5" r="1.4" fill="currentColor" />
      <circle cx="10" cy="10" r="1.4" fill="currentColor" />
      <circle cx="10" cy="15.5" r="1.4" fill="currentColor" />
    </svg>
  );
}

export function IconGrip(props: IconProps) {
  return (
    <svg {...base(props)}>
      <circle cx="7" cy="5.5" r="1.2" fill="currentColor" />
      <circle cx="13" cy="5.5" r="1.2" fill="currentColor" />
      <circle cx="7" cy="10" r="1.2" fill="currentColor" />
      <circle cx="13" cy="10" r="1.2" fill="currentColor" />
      <circle cx="7" cy="14.5" r="1.2" fill="currentColor" />
      <circle cx="13" cy="14.5" r="1.2" fill="currentColor" />
    </svg>
  );
}

export function IconCheck(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M4.5 10.5l3.5 3.5 7.5-8" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconChevronLeft(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M12.5 4.5L7 10l5.5 5.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconSparkle(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path
        d="M10 3l1.4 4.6L16 9l-4.6 1.4L10 15l-1.4-4.6L4 9l4.6-1.4L10 3z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconSend(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M3.5 10l13-6-6 13-1.6-5.4L3.5 10z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

export function IconWifiOff(props: IconProps) {
  return (
    <svg {...base(props)} fill="none">
      <path d="M3 8.5c1.9-1.7 4.3-2.7 7-2.7s5.1 1 7 2.7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M5.8 11.4a6.7 6.7 0 0 1 8.4 0" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="10" cy="14.7" r="1.1" fill="currentColor" />
      <path d="M3 3l14 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

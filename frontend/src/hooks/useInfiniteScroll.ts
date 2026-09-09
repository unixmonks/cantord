import { useEffect, useRef } from "react";

// Calls `onIntersect` whenever the returned sentinel element scrolls into view,
// as long as `enabled` is true. Attach the ref to an empty div near the end
// of a list to trigger loading the next page automatically.
export function useInfiniteScroll(onIntersect: () => void, enabled: boolean) {
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  const onIntersectRef = useRef(onIntersect);
  onIntersectRef.current = onIntersect;

  useEffect(() => {
    if (!enabled) return;
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) onIntersectRef.current();
      },
      { rootMargin: "400px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [enabled]);

  return sentinelRef;
}

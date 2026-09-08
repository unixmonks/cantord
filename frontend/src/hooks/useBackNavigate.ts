import { useNavigate } from "react-router-dom";

/**
 * Returns a "go back" handler that retraces actual in-app navigation
 * history (e.g. Album A -> Artist -> Album B -> Back lands on Artist, not
 * the Albums list), falling back to the given section's list root when
 * there's no prior in-app entry to return to — e.g. the page was opened
 * directly via a bookmarked/typed URL. `history.state.idx` (set by
 * react-router's browser history) is 0 only for the first entry since page
 * load, which is exactly that direct-load case; every client-side
 * navigation before this one bumps it. Scroll position and pagination on
 * the target page are restored independently (see Layout's scroll cache and
 * AlbumsPage's list cache), so either path lands you back where you left it.
 */
export function useBackNavigate(sectionRoot: string) {
  const navigate = useNavigate();
  return () => {
    if ((window.history.state as { idx?: number } | null)?.idx) {
      navigate(-1);
    } else {
      navigate(sectionRoot);
    }
  };
}

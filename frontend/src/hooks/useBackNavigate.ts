import { useNavigate } from "react-router-dom";

/**
 * Returns a "go back" handler that always returns to the given section's
 * list root — e.g. Back on an album detail page goes to the Albums list,
 * no matter how the album was reached (Albums list, sidebar resume, search,
 * an artist's discography, …). Scroll position and pagination on that list
 * page are restored independently (see Layout's scroll cache and
 * AlbumsPage's list cache), so this always lands you back where you left it.
 */
export function useBackNavigate(sectionRoot: string) {
  const navigate = useNavigate();
  return () => navigate(sectionRoot);
}

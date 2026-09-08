import { TrackListPage } from "./TrackListPage";

export function FavoritesPage() {
  return (
    <TrackListPage
      title="Favorites"
      emptyMessage="Tap the heart on any track to favorite it."
      fetcher={(api) => api.listFavorites()}
    />
  );
}

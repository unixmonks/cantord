import { TrackListPage } from "./TrackListPage";

export function RecentlyPlayedPage() {
  return (
    <TrackListPage
      title="Recently Played"
      emptyMessage="Nothing has been played yet."
      fetcher={(api) => api.recentlyPlayed(100)}
    />
  );
}

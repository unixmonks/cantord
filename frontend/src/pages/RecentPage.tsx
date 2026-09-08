import { TrackListPage } from "./TrackListPage";

export function RecentPage() {
  return (
    <TrackListPage
      title="Recently Added"
      emptyMessage="Nothing has been added to the library yet."
      fetcher={(api) => api.recentlyAdded(100)}
    />
  );
}

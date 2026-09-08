import { BrowserRouter, Route, Routes } from "react-router-dom";
import { PlayerProvider } from "./state/PlayerContext";
import { Layout } from "./components/Layout";
import { HomePage } from "./pages/HomePage";
import { QueuePage } from "./pages/QueuePage";
import { AlbumsPage } from "./pages/AlbumsPage";
import { AlbumDetailPage } from "./pages/AlbumDetailPage";
import { ArtistsPage } from "./pages/ArtistsPage";
import { ArtistDetailPage } from "./pages/ArtistDetailPage";
import { GenresPage } from "./pages/GenresPage";
import { GenreDetailPage } from "./pages/GenreDetailPage";
import { SearchPage } from "./pages/SearchPage";
import { PlaylistsPage } from "./pages/PlaylistsPage";
import { PlaylistDetailPage } from "./pages/PlaylistDetailPage";
import { RecentPage } from "./pages/RecentPage";
import { RecentlyPlayedPage } from "./pages/RecentlyPlayedPage";
import { FavoritesPage } from "./pages/FavoritesPage";
import { SettingsPage } from "./pages/SettingsPage";

export default function App() {
  return (
    <PlayerProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/queue" element={<QueuePage />} />
            <Route path="/albums" element={<AlbumsPage />} />
            <Route path="/albums/:id" element={<AlbumDetailPage />} />
            <Route path="/artists" element={<ArtistsPage />} />
            <Route path="/artists/:name" element={<ArtistDetailPage />} />
            <Route path="/genres" element={<GenresPage />} />
            <Route path="/genres/:genre" element={<GenreDetailPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/playlists" element={<PlaylistsPage />} />
            <Route path="/playlists/:name" element={<PlaylistDetailPage />} />
            <Route path="/recent" element={<RecentPage />} />
            <Route path="/recently-played" element={<RecentlyPlayedPage />} />
            <Route path="/favorites" element={<FavoritesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </PlayerProvider>
  );
}

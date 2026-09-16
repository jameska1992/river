import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './context/authContext'
import { BottomTabsLayout } from './components/BottomTabs'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import SearchPage from './pages/SearchPage'
import LibraryPage from './pages/LibraryPage'
import WatchlistPage from './pages/WatchlistPage'
import SettingsPage from './pages/SettingsPage'
import MovieDetailPage from './pages/MovieDetailPage'
import TVShowDetailPage from './pages/TVShowDetailPage'
import AlbumDetailPage from './pages/AlbumDetailPage'
import AudiobookDetailPage from './pages/AudiobookDetailPage'
import CollectionDetailPage from './pages/CollectionDetailPage'
import MoviePlayerPage from './pages/MoviePlayerPage'
import EpisodePlayerPage from './pages/EpisodePlayerPage'
import { AudioPlayerProvider } from './context/AudioPlayerProvider'

export default function App() {
  const { user, isLoading } = useAuth()
  if (isLoading) return null

  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    // The audio player lives above the router so its <audio> and the docked
    // mini-player survive navigation. Mounted here (authenticated only) so its
    // progress socket never opens on the login screen. Fullscreen video routes
    // pause nothing — audio just keeps playing underneath if it was started.
    <AudioPlayerProvider>
      <Routes>
        <Route path="/login" element={<Navigate to="/" replace />} />

        {/* Fullscreen video players (no tab bar). */}
        <Route path="/movies/:id/watch" element={<MoviePlayerPage />} />
        <Route path="/tvshows/:showId/seasons/:seasonId/episodes/:episodeId/watch" element={<EpisodePlayerPage />} />

        {/* Everything else lives under the bottom-tab shell. Audio (music +
            audiobooks) plays via the docked mini/full player, not a route. */}
        <Route element={<BottomTabsLayout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/library" element={<LibraryPage />} />
          <Route path="/watchlist" element={<WatchlistPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/movies/:id" element={<MovieDetailPage />} />
          <Route path="/tvshows/:id" element={<TVShowDetailPage />} />
          <Route path="/albums/:id" element={<AlbumDetailPage />} />
          <Route path="/audiobooks/:id" element={<AudiobookDetailPage />} />
          <Route path="/collections/:id" element={<CollectionDetailPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </AudioPlayerProvider>
  )
}

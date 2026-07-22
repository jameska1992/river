import { Routes, Route, Navigate, Outlet } from 'react-router-dom'
import { HeroBanner } from './components/HeroBanner'
import { ContinueWatching as ContinueWatchingSection } from './components/ContinueWatching'
import { NextUp } from './components/NextUp'
import { LibraryCarousels } from './components/LibraryCarousels'
import { CollectionsCarousel } from './components/CollectionsCarousel'
import { WatchlistCarousel } from './components/WatchlistCarousel'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AdminRoute } from './components/AdminRoute'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { DownloadPage } from './pages/DownloadPage'
import { LibraryPage } from './pages/LibraryPage'
import { MovieDetailPage } from './pages/MovieDetailPage'
import { MovieWatchPage } from './pages/MovieWatchPage'
import { TVShowDetailPage } from './pages/TVShowDetailPage'
import { AudiobookDetailPage } from './pages/AudiobookDetailPage'
import { AudiobookListenPage } from './pages/AudiobookListenPage'
import { ArtistDetailPage } from './pages/ArtistDetailPage'
import { AlbumDetailPage } from './pages/AlbumDetailPage'
import { MusicPlayerPage } from './pages/MusicPlayerPage'
import { EpisodeWatchPage } from './pages/EpisodeWatchPage'
import { PersonDetailPage } from './pages/PersonDetailPage'
import { SearchPage } from './pages/SearchPage'
import { CollectionsPage } from './pages/CollectionsPage'
import { CollectionDetailPage } from './pages/CollectionDetailPage'
import { WatchlistPage } from './pages/WatchlistPage'
import { AdminLayout } from './pages/admin/AdminLayout'
import { OverviewPage } from './pages/admin/OverviewPage'
import { LibrariesPage } from './pages/admin/LibrariesPage'
import { UploadPage } from './pages/admin/UploadPage'
import { UsersPage } from './pages/admin/UsersPage'
import { LogsPage } from './pages/admin/LogsPage'
import { ScannerStatePage } from './pages/admin/ScannerStatePage'
import { UnidentifiedPage } from './pages/admin/UnidentifiedPage'
import { RequestPage } from './pages/RequestPage'
import { CalendarPage } from './pages/CalendarPage'
import { SettingsPage } from './pages/SettingsPage'
import { LibrariesProvider } from './context/LibrariesContext'
import { MoviesProvider } from './context/MoviesContext'
import { TVShowsProvider } from './context/TVShowsContext'
import { MusicProvider } from './context/MusicContext'
import { AudiobooksProvider } from './context/AudiobooksContext'
import { WatchlistProvider } from './context/WatchlistContext'

// Wraps all media contexts around an <Outlet> so both Layout and
// bare routes (e.g. the watch page) share the same provider tree.
function MediaProviders() {
  return (
    <LibrariesProvider>
      <MoviesProvider>
        <TVShowsProvider>
          <MusicProvider>
            <AudiobooksProvider>
              <WatchlistProvider>
                <Outlet />
              </WatchlistProvider>
            </AudiobooksProvider>
          </MusicProvider>
        </TVShowsProvider>
      </MoviesProvider>
    </LibrariesProvider>
  )
}

function HomePage() {
  return (
    <>
      <HeroBanner />
      <div className="container" style={{ paddingTop: 'var(--space-5)' }}>
        <NextUp />
        <ContinueWatchingSection />
        <WatchlistCarousel />
        <CollectionsCarousel />
        <LibraryCarousels />
      </div>
    </>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/download" element={<DownloadPage />} />

      <Route element={<ProtectedRoute />}>
        <Route element={<MediaProviders />}>
          {/* Full-screen watch/listen pages — no navbar */}
          <Route path="/movie/:id/watch" element={<MovieWatchPage />} />
          <Route path="/show/:showId/season/:seasonId/episode/:episodeId/watch" element={<EpisodeWatchPage />} />
          <Route path="/audiobook/:audiobookId/listen" element={<AudiobookListenPage />} />
          <Route path="/album/:albumId/play" element={<MusicPlayerPage />} />

          {/* Standard pages inside the Layout shell */}
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/library/:id" element={<LibraryPage />} />
            <Route path="/movie/:id" element={<MovieDetailPage />} />
            <Route path="/show/:id" element={<TVShowDetailPage />} />
            <Route path="/audiobook/:id" element={<AudiobookDetailPage />} />
            <Route path="/artist/:id" element={<ArtistDetailPage />} />
            <Route path="/album/:id" element={<AlbumDetailPage />} />
            <Route path="/person/:id" element={<PersonDetailPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/collections" element={<CollectionsPage />} />
            <Route path="/collections/:id" element={<CollectionDetailPage />} />
            <Route path="/watchlist" element={<WatchlistPage />} />
            <Route path="/request" element={<RequestPage />} />
            <Route path="/calendar" element={<CalendarPage />} />
            <Route path="/settings" element={<SettingsPage />} />

            <Route element={<AdminRoute />}>
              <Route path="/admin" element={<AdminLayout />}>
                <Route index element={<OverviewPage />} />
                <Route path="libraries" element={<LibrariesPage />} />
                <Route path="upload" element={<UploadPage />} />
                <Route path="unidentified" element={<UnidentifiedPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="logs" element={<LogsPage />} />
                <Route path="scanner-state" element={<ScannerStatePage />} />
              </Route>
            </Route>

            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  )
}

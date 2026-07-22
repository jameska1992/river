import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import MoviesPage from './pages/MoviesPage'
import TVShowsPage from './pages/TVShowsPage'
import CollectionsPage from './pages/CollectionsPage'
import CollectionDetailPage from './pages/CollectionDetailPage'
import AudiobooksPage from './pages/AudiobooksPage'
import AudiobookDetailPage from './pages/AudiobookDetailPage'
import AudiobookPlayerPage from './pages/AudiobookPlayerPage'
import SearchPage from './pages/SearchPage'
import WatchlistPage from './pages/WatchlistPage'
import MovieDetailPage from './pages/MovieDetailPage'
import MoviePlayerPage from './pages/MoviePlayerPage'
import TVShowDetailPage from './pages/TVShowDetailPage'
import EpisodePlayerPage from './pages/EpisodePlayerPage'
import PersonDetailPage from './pages/PersonDetailPage'
import type { ReactNode } from 'react'

function Protected({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth()
  if (isLoading) return null
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  const { user, isLoading } = useAuth()
  if (isLoading) return null

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <LoginPage />} />
      <Route path="/" element={<Protected><HomePage /></Protected>} />
      <Route path="/movies" element={<Protected><MoviesPage /></Protected>} />
      <Route path="/movies/:id" element={<Protected><MovieDetailPage /></Protected>} />
      <Route path="/movies/:id/watch" element={<Protected><MoviePlayerPage /></Protected>} />
      <Route path="/people/:id" element={<Protected><PersonDetailPage /></Protected>} />
      <Route path="/tvshows" element={<Protected><TVShowsPage /></Protected>} />
      <Route path="/tvshows/:id" element={<Protected><TVShowDetailPage /></Protected>} />
      <Route
        path="/tvshows/:showId/seasons/:seasonId/episodes/:episodeId/watch"
        element={<Protected><EpisodePlayerPage /></Protected>}
      />
      <Route path="/collections" element={<Protected><CollectionsPage /></Protected>} />
      <Route path="/search" element={<Protected><SearchPage /></Protected>} />
      <Route path="/watchlist" element={<Protected><WatchlistPage /></Protected>} />
      <Route path="/collections/:id" element={<Protected><CollectionDetailPage /></Protected>} />
      <Route path="/audiobooks" element={<Protected><AudiobooksPage /></Protected>} />
      <Route path="/audiobooks/:id" element={<Protected><AudiobookDetailPage /></Protected>} />
      <Route
        path="/audiobooks/:audiobookId/chapters/:chapterId/listen"
        element={<Protected><AudiobookPlayerPage /></Protected>}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

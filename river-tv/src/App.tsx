import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useAuth } from './context/authContext'
import LoginPage from './pages/LoginPage'
import AccountPickerPage from './pages/AccountPickerPage'
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
  const { user, isLoading, accounts } = useAuth()
  if (isLoading) return null
  // No active session: show the account picker when this TV remembers any
  // accounts, otherwise the login form.
  if (!user) return <Navigate to={accounts.length > 0 ? '/accounts' : '/login'} replace />
  return <>{children}</>
}

// The /login route. Normally a signed-in user is bounced home (so Back can't
// strand them on the login form), but the account picker's "Add account" tile
// navigates here with state.add to sign in a *different* account while a
// session is still active — that intent must render the form, not redirect.
function LoginRoute() {
  const { user } = useAuth()
  const addIntent = (useLocation().state as { add?: boolean } | null)?.add === true
  if (user && !addIntent) return <Navigate to="/" replace />
  return <LoginPage />
}

export default function App() {
  const { isLoading, accounts } = useAuth()
  if (isLoading) return null

  return (
    <Routes>
      <Route path="/login" element={<LoginRoute />} />
      {/* The picker is reachable while signed in too (Sidebar → Change
          account), so it isn't gated on `user`. With no accounts left there's
          nothing to pick — fall through to the login form. */}
      <Route
        path="/accounts"
        element={accounts.length > 0 ? <AccountPickerPage /> : <Navigate to="/login" replace />}
      />
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

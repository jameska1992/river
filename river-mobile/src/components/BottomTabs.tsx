import { NavLink, Outlet } from 'react-router-dom'
import {
  RiHome2Line, RiSearchLine, RiFilmLine, RiBookmarkLine, RiSettings3Line,
} from 'react-icons/ri'
import type { ReactNode } from 'react'
import { useAudioPlayer } from '../context/audioPlayerContext'
import { MiniPlayer } from './MiniPlayer'

// Mini-player height (kept in sync with MiniPlayer's own styling) so content
// can reserve space for it when something is playing.
const MINI_PLAYER_H = '3.75rem'

// Bottom tab bar — the primary phone navigation. Fixed to the bottom with a
// safe-area inset; page content scrolls above it (see the .screen padding).
const TABS: { to: string; label: string; icon: ReactNode; end?: boolean }[] = [
  { to: '/search', label: 'Search', icon: <RiSearchLine /> },
  { to: '/library', label: 'Library', icon: <RiFilmLine /> },
  { to: '/', label: 'Home', icon: <RiHome2Line />, end: true },
  { to: '/watchlist', label: 'Watchlist', icon: <RiBookmarkLine /> },
  { to: '/settings', label: 'Settings', icon: <RiSettings3Line /> },
]

export function BottomTabsLayout() {
  const { current } = useAudioPlayer()
  // When the mini-player is docked it sits above the tab bar, so reserve its
  // height on top of the tab-bar inset to keep page content clear of both.
  const contentPad = current
    ? `calc(var(--tabbar-h) + var(--safe-bottom) + ${MINI_PLAYER_H})`
    : 'calc(var(--tabbar-h) + var(--safe-bottom))'
  return (
    <div style={styles.root}>
      <main style={{ ...styles.content, paddingBottom: contentPad }}>
        <Outlet />
      </main>
      <MiniPlayer />
      <nav style={styles.bar}>
        {TABS.map(t => (
          <NavLink
            key={t.to}
            to={t.to}
            end={t.end}
            style={({ isActive }) => ({
              ...styles.tab,
              color: isActive ? 'var(--accent)' : 'var(--text-muted)',
            })}
          >
            <span style={styles.tabIcon}>{t.icon}</span>
            <span style={styles.tabLabel}>{t.label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  root: { minHeight: '100vh' },
  content: {
    // Reserve space for the fixed tab bar (+ gesture inset) so nothing hides
    // beneath it.
    paddingBottom: 'calc(var(--tabbar-h) + var(--safe-bottom))',
  },
  bar: {
    position: 'fixed',
    left: 0,
    right: 0,
    bottom: 0,
    height: 'calc(var(--tabbar-h) + var(--safe-bottom))',
    paddingBottom: 'var(--safe-bottom)',
    display: 'flex',
    alignItems: 'stretch',
    background: 'var(--bg-elev)',
    borderTop: '1px solid var(--outline)',
    zIndex: 50,
  },
  tab: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '0.2rem',
    fontSize: '1.35rem',
  },
  tabIcon: { display: 'flex' },
  tabLabel: { fontSize: '0.7rem', fontWeight: 600 },
}

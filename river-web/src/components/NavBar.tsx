import { type FormEvent, useEffect, useRef, useState } from 'react'
import { Link, NavLink, useNavigate, useLocation } from 'react-router-dom'
import {
  RiPlayCircleFill,
  RiFilmLine,
  RiTv2Line,
  RiMusicLine,
  RiHeadphoneLine,
  RiFoldersLine,
  RiBookmarkLine,
  RiAddCircleLine,
  RiCalendarEventLine,
  RiSearchLine,
  RiShieldLine,
  RiSettings3Line,
  RiLogoutBoxRLine,
  RiMenuLine,
  RiCloseLine,
} from 'react-icons/ri'
import { useAuth } from '../context/AuthContext'
import { useLibraries } from '../context/LibrariesContext'
import type { LibraryType } from '../api'
import styles from './NavBar.module.css'

const libraryIcons: Record<LibraryType, React.ReactNode> = {
  movie:     <RiFilmLine />,
  tvshow:    <RiTv2Line />,
  music:     <RiMusicLine />,
  audiobook: <RiHeadphoneLine />,
}

export function NavBar() {
  const { user, logout } = useAuth()
  const { libraries, fetch: fetchLibraries } = useLibraries()

  const navigate = useNavigate()
  const location = useLocation()
  const [query, setQuery] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    void fetchLibraries()
  }, [fetchLibraries])

  // Keep the search input in sync when navigating to /search directly or via back/forward.
  useEffect(() => {
    if (location.pathname === '/search') {
      const q = new URLSearchParams(location.search).get('q') ?? ''
      // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs the search input to the URL query on navigation (external route state)
      setQuery(q)
    }
  }, [location])

  // Close mobile menu on route change
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- closes the mobile menu in response to route (external navigation) changes
    setMobileOpen(false)
  }, [location.pathname])

  // Lock body scroll when mobile menu is open
  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : ''
    return () => { document.body.style.overflow = '' }
  }, [mobileOpen])

  // Close desktop user menu on outside click
  useEffect(() => {
    if (!menuOpen) return
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [menuOpen])

  function handleSearch(e: FormEvent) {
    e.preventDefault()
    const q = query.trim()
    if (q) navigate(`/search?q=${encodeURIComponent(q)}`)
  }

  async function handleLogout() {
    setMenuOpen(false)
    setMobileOpen(false)
    await logout()
    navigate('/login', { replace: true })
  }

  const initials = user ? user.username.slice(0, 2).toUpperCase() : '??'

  const navLinks = (
    <>
      {libraries.map(lib => (
        <NavLink
          key={lib.id}
          to={`/library/${lib.id}`}
          className={({ isActive }) =>
            `${styles.libraryLink} ${isActive ? styles.libraryLinkActive : ''}`
          }
        >
          <span className={styles.libraryIcon} aria-hidden>{libraryIcons[lib.type]}</span>
          {lib.name}
        </NavLink>
      ))}
      <NavLink
        to="/collections"
        className={({ isActive }) =>
          `${styles.libraryLink} ${isActive ? styles.libraryLinkActive : ''}`
        }
      >
        <span className={styles.libraryIcon} aria-hidden><RiFoldersLine /></span>
        Collections
      </NavLink>
      <NavLink
        to="/watchlist"
        className={({ isActive }) =>
          `${styles.libraryLink} ${isActive ? styles.libraryLinkActive : ''}`
        }
      >
        <span className={styles.libraryIcon} aria-hidden><RiBookmarkLine /></span>
        Watchlist
      </NavLink>
      <NavLink
        to="/request"
        className={({ isActive }) =>
          `${styles.libraryLink} ${isActive ? styles.libraryLinkActive : ''}`
        }
      >
        <span className={styles.libraryIcon} aria-hidden><RiAddCircleLine /></span>
        Request
      </NavLink>
      <NavLink
        to="/calendar"
        className={({ isActive }) =>
          `${styles.libraryLink} ${isActive ? styles.libraryLinkActive : ''}`
        }
      >
        <span className={styles.libraryIcon} aria-hidden><RiCalendarEventLine /></span>
        Calendar
      </NavLink>
    </>
  )

  return (
    <>
      <header className={`glass-nav ${styles.navbar}`}>
        <div className={`container ${styles.inner}`}>

          {/* Logo */}
          <Link to="/" className={styles.logo} aria-label="River home">
            <RiPlayCircleFill className={styles.logoIcon} />
            <span>River</span>
          </Link>

          {/* Desktop nav links */}
          <nav className={styles.libraries} aria-label="Libraries">
            {navLinks}
          </nav>

          {/* Desktop search */}
          <form onSubmit={handleSearch} className={styles.searchForm} role="search">
            <RiSearchLine className={styles.searchIcon} aria-hidden />
            <input
              type="search"
              className={`input ${styles.searchInput}`}
              placeholder="Search…"
              value={query}
              onChange={e => setQuery(e.target.value)}
              aria-label="Search"
            />
          </form>

          {/* Desktop user avatar */}
          <div className={`${styles.actions} ${styles.desktopActions}`}>
            <div className={styles.userWrap} ref={menuRef}>
              <button
                className={styles.avatar}
                onClick={() => setMenuOpen(v => !v)}
                aria-label="User menu"
                aria-expanded={menuOpen}
              >
                {initials}
              </button>

              {menuOpen && (
                <div className={`glass ${styles.userMenu}`} role="menu">
                  <div className={styles.userInfo}>
                    <span className="label-md">{user?.username}</span>
                    <span className="label-sm">{user?.email}</span>
                  </div>
                  <div className="divider" />
                  <Link
                    to="/settings"
                    className={styles.menuItem}
                    role="menuitem"
                    onClick={() => setMenuOpen(false)}
                  >
                    <RiSettings3Line size={16} />
                    Settings
                  </Link>
                  {user?.role === 'admin' && (
                    <Link
                      to="/admin"
                      className={styles.menuItem}
                      role="menuitem"
                      onClick={() => setMenuOpen(false)}
                    >
                      <RiShieldLine size={16} />
                      Admin Panel
                    </Link>
                  )}
                  <div className="divider" />
                  <button
                    className={styles.menuItem}
                    onClick={handleLogout}
                    role="menuitem"
                  >
                    <RiLogoutBoxRLine size={16} />
                    Sign out
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Mobile hamburger */}
          <button
            className={styles.hamburger}
            onClick={() => setMobileOpen(v => !v)}
            aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={mobileOpen}
          >
            {mobileOpen ? <RiCloseLine size={22} /> : <RiMenuLine size={22} />}
          </button>

        </div>
      </header>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className={`glass-nav ${styles.mobileMenu}`}>
          <div className={`container ${styles.mobileInner}`}>

            {/* Search */}
            <form onSubmit={handleSearch} className={styles.mobileSearchForm} role="search">
              <RiSearchLine className={styles.mobileSearchIcon} aria-hidden />
              <input
                type="search"
                className={`input ${styles.mobileSearchInput}`}
                placeholder="Search…"
                value={query}
                onChange={e => setQuery(e.target.value)}
                aria-label="Search"
              />
            </form>

            {/* Nav links */}
            <nav className={styles.mobileNav} aria-label="Libraries">
              {navLinks}
            </nav>

            <div className="divider" />

            {/* User section */}
            <div className={styles.mobileUserInfo}>
              <span className="label-md">{user?.username}</span>
              <span className="label-sm" style={{ color: 'var(--color-on-surface-variant)' }}>{user?.email}</span>
            </div>

            <Link
              to="/settings"
              className={styles.menuItem}
              onClick={() => setMobileOpen(false)}
            >
              <RiSettings3Line size={16} />
              Settings
            </Link>

            {user?.role === 'admin' && (
              <Link
                to="/admin"
                className={styles.menuItem}
                onClick={() => setMobileOpen(false)}
              >
                <RiShieldLine size={16} />
                Admin Panel
              </Link>
            )}

            <button className={styles.menuItem} onClick={handleLogout}>
              <RiLogoutBoxRLine size={16} />
              Sign out
            </button>

          </div>
        </div>
      )}
    </>
  )
}

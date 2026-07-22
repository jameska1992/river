import { useCallback, useState, type ReactNode } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  RiBookmarkLine,
  RiFilmLine,
  RiFoldersLine,
  RiHeadphoneLine,
  RiHome2Line,
  RiLogoutBoxRLine,
  RiPlayCircleFill,
  RiSearchLine,
  RiTv2Line,
} from 'react-icons/ri'
import { useAuth } from '../context/authContext'
import { useFocusable } from '../hooks/useFocus'

/*
 * Collapsible sidebar. Collapsed = a narrow icon rail; expanded = wider
 * with labels. Auto-expands while any item inside it has focus, auto-
 * collapses when focus moves away. Pages reserve `--sidebar-rail` of
 * left padding so the rail is never overlapped — the expanded sidebar
 * floats over content for the extra width without shifting layout.
 */

interface NavItem {
  label: string
  to: string
  icon: ReactNode
}

// Icons mirror river-web's NavBar so the two surfaces feel like the
// same product. If web swaps an icon, swap it here too.
const ITEMS: NavItem[] = [
  { label: 'Home',        to: '/',            icon: <RiHome2Line />      },
  { label: 'Search',      to: '/search',      icon: <RiSearchLine />     },
  { label: 'Movies',      to: '/movies',      icon: <RiFilmLine />       },
  { label: 'TV Shows',    to: '/tvshows',     icon: <RiTv2Line />        },
  { label: 'Audiobooks',  to: '/audiobooks',  icon: <RiHeadphoneLine />  },
  { label: 'Collections', to: '/collections', icon: <RiFoldersLine />    },
  { label: 'Watchlist',   to: '/watchlist',   icon: <RiBookmarkLine />   },
]

export const SIDEBAR_RAIL_REM = 5
export const SIDEBAR_EXPANDED_REM = 16

export function Sidebar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const [focusCount, setFocusCount] = useState(0)

  // One callback shared by all items — increments on focus-in, decrements
  // on focus-out. Two-callback transitions within the sidebar net to +1,
  // moving focus out lands at 0. (apply() runs prev-out before next-in,
  // and React batches both setState calls into one render, so we never
  // briefly observe 0 mid-transition.)
  const handleFocusChange = useCallback((focused: boolean) => {
    setFocusCount(c => focused ? c + 1 : Math.max(0, c - 1))
  }, [])

  const expanded = focusCount > 0
  const width = expanded ? `${SIDEBAR_EXPANDED_REM}rem` : `${SIDEBAR_RAIL_REM}rem`

  return (
    <aside style={{ ...styles.aside, width }}>
      <div style={styles.brand}>
        <span style={styles.brandIcon}><RiPlayCircleFill /></span>
        <span style={{ ...styles.brandLabel, ...(expanded ? styles.labelVisible : styles.labelHidden) }}>
          River
        </span>
      </div>

      <nav style={styles.nav}>
        {ITEMS.map(item => (
          <Item
            key={item.to}
            label={item.label}
            icon={item.icon}
            active={pathname === item.to}
            expanded={expanded}
            onSelect={() => navigate(item.to)}
            onFocusChange={handleFocusChange}
          />
        ))}
      </nav>

      <div style={styles.footer}>
        <Item
          label={user?.username ?? 'Sign out'}
          icon={<RiLogoutBoxRLine />}
          active={false}
          expanded={expanded}
          onSelect={() => void logout()}
          onFocusChange={handleFocusChange}
        />
      </div>
    </aside>
  )
}

interface ItemProps {
  label: string
  icon: ReactNode
  active: boolean
  expanded: boolean
  onSelect: () => void
  onFocusChange: (focused: boolean) => void
}

function Item({ label, icon, active, expanded, onSelect, onFocusChange }: ItemProps) {
  // Track focus locally so we can override the global .focused styles
  // (outset box-shadow + 1.05 scale) — those bleed past the sidebar
  // edge on a 5rem rail. Inset ring + no-scale keeps the indicator
  // visible while staying inside the aside bounds.
  const [focused, setFocused] = useState(false)
  // skipInitialFocus — without it, the first-mounted sidebar item would
  // claim focus on page load, expand the sidebar, and then collapse it
  // again when the page's autofocus card steals focus a moment later.
  // group: 'sidebar' — locks up/down/left to other sidebar items, so the
  // expanded sidebar's hit-boxes can't accidentally pull focus into the
  // page. Right is the explicit way out.
  // tag: 'sidebar' — hides sidebar items from spatial Up/Down/Right from
  // page items. They are only reachable via an explicit override target
  // (e.g. left from the first card of a row) or the Back key.
  const ref = useFocusable<HTMLButtonElement>(
    onSelect,
    {
      skipInitialFocus: true,
      group: 'sidebar',
      tag: 'sidebar',
      spatialHidden: true,
      onFocusChange: f => {
        setFocused(f)
        onFocusChange(f)
      },
    },
  )
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{
        ...styles.item,
        ...(active ? styles.itemActive : {}),
        ...(focused ? styles.itemFocused : {}),
      }}
    >
      <span style={styles.itemIcon}>{icon}</span>
      <span style={{ ...styles.itemLabel, ...(expanded ? styles.labelVisible : styles.labelHidden) }}>
        {label}
      </span>
    </button>
  )
}

const labelTransition = 'opacity 180ms ease, transform 180ms ease'

const styles: Record<string, React.CSSProperties> = {
  aside: {
    position: 'fixed',
    top: 0,
    left: 0,
    bottom: 0,
    background: 'var(--bg-elev)',
    borderRight: '1px solid rgba(255,255,255,0.04)',
    display: 'flex',
    flexDirection: 'column',
    // No `overflow: hidden` — the focus ring is a box-shadow that
    // extends outside the focused item, and clipping the sidebar would
    // make it invisible. Labels are clipped inside each item via flex
    // shrinking + overflow:hidden on the label span instead.
    transition: 'width 180ms ease',
    zIndex: 50,
  },
  brand: {
    display: 'flex',
    alignItems: 'center',
    padding: '1.5rem 0',
    fontFamily: 'var(--font-logo)',
    fontSize: '1.5rem',
    fontWeight: 700,
    color: 'var(--text)',
  },
  brandIcon: {
    width: `${SIDEBAR_RAIL_REM}rem`,
    height: '3rem',
    display: 'grid',
    placeItems: 'center',
    fontSize: '2rem',
    color: 'var(--accent)',
    flexShrink: 0,
  },
  brandLabel: {
    fontFamily: 'var(--font-logo)',
    fontSize: '1.6rem',
    fontWeight: 700,
    transition: labelTransition,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    flex: '1 1 0',
    minWidth: 0,
  },
  nav: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
    paddingTop: '1rem',
  },
  footer: {
    paddingBottom: '1rem',
  },
  item: {
    display: 'flex',
    alignItems: 'center',
    width: '100%',
    color: 'var(--text-muted)',
    padding: 0,
    background: 'transparent',
    textAlign: 'left',
    borderRadius: 0,
  },
  itemActive: {
    color: 'var(--text)',
    background: 'var(--accent-soft)',
  },
  itemFocused: {
    // Inline overrides for the global .focused class: replace the
    // outset ring (which would extend past the sidebar edge) with an
    // inset ring drawn at the right edge, and skip the scale-up that
    // also bleeds. Inset rings render inside the element's box so
    // they're always clipped by the sidebar regardless of width.
    boxShadow: 'inset -0.25rem 0 0 0 var(--focus)',
    background: 'var(--accent-soft)',
    color: 'var(--text)',
    transform: 'none',
  },
  itemIcon: {
    width: `${SIDEBAR_RAIL_REM}rem`,
    height: '3.25rem',
    display: 'grid',
    placeItems: 'center',
    fontSize: '1.5rem',
    flexShrink: 0,
  },
  itemLabel: {
    fontSize: '1.05rem',
    transition: labelTransition,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    // flex shrink + minWidth: 0 lets the label collapse to zero width
    // when the sidebar is collapsed, so the label doesn't push the item
    // wider than the rail (which would otherwise need clipping at the
    // aside level — and that clipping also kills the focus ring).
    flex: '1 1 0',
    minWidth: 0,
  },
  labelHidden: {
    opacity: 0,
    transform: 'translateX(-0.5rem)',
    pointerEvents: 'none',
  },
  labelVisible: {
    opacity: 1,
    transform: 'translateX(0)',
  },
}

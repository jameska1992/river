import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import type { SortOrder } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { PosterCard } from '../components/PosterCard'
import { Sidebar } from '../components/Sidebar'
import { Popup } from '../components/Popup'

// PAGE_SIZE is paired with the grid below: 100 = 10 cols × 10 rows so
// the final row is always full, regardless of viewport (no trailing gap
// at 1080p or 4K). The server's max page size is 200 (see river-api
// services/helpers.go), so this stays well under the cap.
const PAGE_SIZE = 100
const COLS = 10

// Default whitelist covers movies + tvshows (see river-api
// movieSortColumns / tvShowSortColumns): title, year, rating, added.
// Other resources (audiobooks, music) pass their own list via the
// `sortOptions` prop because the server-side whitelist differs.
export interface SortOption {
  label: string
  sort: string
  order: SortOrder
}
const DEFAULT_SORT_OPTIONS: SortOption[] = [
  { label: 'Title (A–Z)',      sort: 'title',  order: 'asc'  },
  { label: 'Title (Z–A)',      sort: 'title',  order: 'desc' },
  { label: 'Year (newest)',    sort: 'year',   order: 'desc' },
  { label: 'Year (oldest)',    sort: 'year',   order: 'asc'  },
  { label: 'Rating (highest)', sort: 'rating', order: 'desc' },
  { label: 'Rating (lowest)',  sort: 'rating', order: 'asc'  },
  { label: 'Recently added',   sort: 'added',  order: 'desc' },
]

export interface BrowseCard {
  key: string
  title: string
  subtitle?: string
  imageSrc?: string
  onSelect?: () => void
}

interface Props<T> {
  title: string
  countSuffix: string
  sortPrefKey: string
  // Per-resource sort whitelist; falls back to the movies/tvshows set.
  sortOptions?: SortOption[]
  // Aspect ratio for the cards in the grid. Default 'portrait'.
  cardAspect?: 'portrait' | 'landscape' | 'square'
  fetchPage: (
    page: number,
    limit: number,
    sort: string,
    order: SortOrder,
  ) => Promise<{ items: T[]; total: number }>
  toCard: (item: T) => BrowseCard
}

export function BrowsePage<T>({
  title, countSuffix, sortPrefKey,
  sortOptions = DEFAULT_SORT_OPTIONS,
  cardAspect,
  fetchPage, toCard,
}: Props<T>) {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  // Seed from the URL so pressing Back out of a detail page (navigate(-1))
  // returns to the page the user was on instead of resetting to 1.
  const [page, setPage] = useState(() => Math.max(1, Number(searchParams.get('page')) || 1))
  const [sortIdx, setSortIdx] = useState<number>(() => {
    const stored = Number(localStorage.getItem(sortPrefKey))
    return Number.isInteger(stored) && stored >= 0 && stored < sortOptions.length ? stored : 0
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sortOpen, setSortOpen] = useState(false)
  const mainRef = useRef<HTMLElement>(null)

  const sortOpt = sortOptions[sortIdx]

  useEffect(() => {
    let alive = true
    setLoading(true)
    fetchPage(page, PAGE_SIZE, sortOpt.sort, sortOpt.order)
      .then(res => {
        if (!alive) return
        setItems(res.items)
        setTotal(res.total)
        mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
      })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [fetchPage, page, sortOpt.sort, sortOpt.order])

  // Mirror the current page into the URL (?page=N, page 1 clears it) with
  // replace so each pager click doesn't stack a history entry. pickSort resets
  // page to 1, which clears the param.
  useEffect(() => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (page <= 1) next.delete('page')
      else next.set('page', String(page))
      return next
    }, { replace: true })
  }, [page, setSearchParams])

  const pickSort = (next: number) => {
    setSortOpen(false)
    if (next === sortIdx) return
    setSortIdx(next)
    setPage(1)
    localStorage.setItem(sortPrefKey, String(next))
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const canPrev = page > 1
  const canNext = page < totalPages

  return (
    <FocusProvider onBack={() => navigate('/')}>
      <div style={styles.page}>
        <Sidebar />
        <main ref={mainRef} style={styles.main}>
          {error && <div style={styles.error}>{error}</div>}

          <div style={styles.heading}>
            <h2 style={styles.title}>{title}</h2>
            <span style={styles.count}>{total ? `${total} ${countSuffix}` : ''}</span>
            <div style={styles.headingSpacer} />
            <SortButton
              label={sortOpt.label}
              onSelect={() => setSortOpen(true)}
              onFocus={() => mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' })}
            />
          </div>

          <div style={styles.pagerTop}>
            <PagerButton
              kind="prev"
              place="top"
              label="◀"
              ariaLabel="Previous page"
              disabled={!canPrev || loading}
              onSelect={() => setPage(p => Math.max(1, p - 1))}
              onFocus={() => mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' })}
            />
            <span style={styles.pagerStatus}>
              {loading ? 'Loading…' : `Page ${page} of ${totalPages}`}
            </span>
            <PagerButton
              kind="next"
              place="top"
              label="▶"
              ariaLabel="Next page"
              disabled={!canNext || loading}
              onSelect={() => setPage(p => Math.min(totalPages, p + 1))}
              onFocus={() => mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' })}
            />
          </div>

          <div style={styles.grid}>
            {items.map((item, i) => {
              const card = toCard(item)
              const row = Math.floor(i / COLS)
              const lastRow = Math.floor((items.length - 1) / COLS)
              return (
                <PosterCard
                  key={card.key}
                  title={card.title}
                  subtitle={card.subtitle}
                  imageSrc={card.imageSrc}
                  aspect={cardAspect}
                  fill
                  autoFocus={i === 0}
                  overrides={{
                    up:   row === 0 ? 'pager-next-top' : undefined,
                    down: row === lastRow ? 'pager-next' : undefined,
                    left: (i % COLS) === 0 ? 'sidebar' : undefined,
                  }}
                  // Top-row focus snaps the page back to the very top so
                  // the heading + sort button are visible even after the
                  // user has scrolled mid-grid via earlier focus moves.
                  onFocus={row === 0 ? () => mainRef.current?.scrollTo({ top: 0, behavior: 'smooth' }) : undefined}
                  onSelect={card.onSelect}
                />
              )
            })}
          </div>

          <div style={styles.pager}>
            <PagerButton
              kind="prev"
              label="◀"
              ariaLabel="Previous page"
              disabled={!canPrev || loading}
              onSelect={() => setPage(p => Math.max(1, p - 1))}
              onFocus={() => mainRef.current?.scrollTo({ top: mainRef.current.scrollHeight, behavior: 'smooth' })}
            />
            <span style={styles.pagerStatus}>
              {loading ? 'Loading…' : `Page ${page} of ${totalPages}`}
            </span>
            <PagerButton
              kind="next"
              label="▶"
              ariaLabel="Next page"
              disabled={!canNext || loading}
              onSelect={() => setPage(p => Math.min(totalPages, p + 1))}
              onFocus={() => mainRef.current?.scrollTo({ top: mainRef.current.scrollHeight, behavior: 'smooth' })}
            />
          </div>
        </main>

        {sortOpen && (
          <Popup onClose={() => setSortOpen(false)}>
            <h3 style={styles.popupTitle}>Sort by</h3>
            <div style={styles.popupList}>
              {sortOptions.map((opt, i) => (
                <SortOptionRow
                  key={opt.label}
                  label={opt.label}
                  active={i === sortIdx}
                  onSelect={() => pickSort(i)}
                />
              ))}
            </div>
          </Popup>
        )}
      </div>
    </FocusProvider>
  )
}

function SortOptionRow({ label, active, onSelect }: { label: string; active: boolean; onSelect: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect)
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={{ ...styles.popupRow, ...(active ? styles.popupRowActive : {}) }}
    >
      <span>{label}</span>
      {active && <span style={styles.popupRowCheck}>✓</span>}
    </button>
  )
}

function SortButton({ label, onSelect, onFocus }: {
  label: string
  onSelect: () => void
  onFocus?: () => void
}) {
  const ref = useFocusable<HTMLButtonElement>(
    onSelect,
    { tag: 'sort', spatialHidden: true, onFocusChange: focused => { if (focused) onFocus?.() } },
  )
  return (
    <button ref={ref} tabIndex={-1} onClick={onSelect} style={styles.sortBtn}>
      <span style={styles.sortBtnLabel}>Sort</span>
      <span style={styles.sortBtnValue}>{label}</span>
    </button>
  )
}

function PagerButton({ kind, place = 'bottom', label, ariaLabel, disabled, onSelect, onFocus }: {
  kind: 'prev' | 'next'
  place?: 'top' | 'bottom'
  label: string
  ariaLabel?: string
  disabled: boolean
  onSelect: () => void
  onFocus?: () => void
}) {
  // Each pager button gets its own tag (suffixed per row) so card overrides
  // can target a specific one — Down from the last row → 'pager-next', Up from
  // the first row → 'pager-next-top'. Each button declares an override toward
  // its sibling so Left/Right between them still works despite spatialHidden
  // cutting them off from spatial nav; the top pair also maps Up to the Sort
  // button in the heading.
  const suffix = place === 'top' ? '-top' : ''
  const tag = `pager-${kind}${suffix}`
  const overrides = {
    ...(kind === 'next' ? { left: `pager-prev${suffix}` } : { right: `pager-next${suffix}` }),
    ...(place === 'top' ? { up: 'sort' } : {}),
  }
  const ref = useFocusable<HTMLButtonElement>(
    disabled ? undefined : onSelect,
    {
      tag,
      spatialHidden: true,
      overrides,
      onFocusChange: focused => { if (focused) onFocus?.() },
    },
  )
  return (
    <button
      ref={ref}
      tabIndex={-1}
      disabled={disabled}
      aria-label={ariaLabel ?? label}
      onClick={disabled ? undefined : onSelect}
      style={{ ...styles.pagerBtn, ...(disabled ? styles.pagerBtnDisabled : {}) }}
    >
      {label}
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    paddingLeft: 'var(--sidebar-rail)',
  },
  main: {
    flex: 1,
    overflowY: 'auto',
    // Extra top/bottom over --safe-y so the title and pager don't sit
    // tight against the TV-safe edge.
    padding: 'calc(var(--safe-y) + 1.5rem) var(--safe-x) calc(var(--safe-y) + 1.5rem)',
    scrollbarWidth: 'none',
  },
  heading: {
    display: 'flex',
    alignItems: 'center',
    gap: '1rem',
    marginBottom: '1.5rem',
  },
  headingSpacer: { flex: 1 },
  sortBtn: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.6rem',
    background: 'var(--bg-elev)',
    padding: '0.6rem 1.1rem',
    borderRadius: 'var(--radius-md)',
  },
  sortBtnLabel: {
    color: 'var(--text-muted)',
    fontSize: '0.85rem',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
  },
  sortBtnValue: {
    color: 'var(--text)',
    fontSize: '1rem',
    fontWeight: 600,
  },
  title: {
    margin: 0,
    fontSize: '1.75rem',
    fontWeight: 600,
  },
  count: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(10, minmax(0, 1fr))',
    gap: '1.5rem 1.25rem',
  },
  pager: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '2rem',
    padding: '2.5rem 0',
  },
  pagerTop: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '2rem',
    padding: '0 0 1.5rem',
  },
  pagerBtn: {
    background: 'var(--bg-elev)',
    color: 'var(--text)',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.25rem',
    fontWeight: 700,
    display: 'grid',
    placeItems: 'center',
    width: '3.5rem',
    height: '3.5rem',
    lineHeight: 1,
  },
  pagerBtnDisabled: {
    opacity: 0.4,
    cursor: 'default',
  },
  pagerStatus: {
    color: 'var(--text-muted)',
    fontSize: '1.05rem',
    minWidth: '10rem',
    textAlign: 'center',
  },
  error: {
    margin: '0 0 1.5rem',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
  popupTitle: {
    margin: '0 0 1rem',
    fontSize: '1.25rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
  },
  popupList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },
  popupRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.9rem 1.25rem',
    fontSize: '1.05rem',
    color: 'var(--text)',
    borderRadius: 'var(--radius-md)',
    background: 'transparent',
    textAlign: 'left',
  },
  popupRowActive: {
    background: 'var(--accent-soft)',
    fontWeight: 600,
  },
  popupRowCheck: {
    color: 'var(--accent)',
    fontSize: '1.1rem',
  },
}

import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type Person } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Sidebar } from '../components/Sidebar'
import { PosterCard } from '../components/PosterCard'
import { imageUrl } from '../util/imageUrl'

const COLS = 10

interface MediaEntry {
  id: string
  title: string
  year: number
  posterPath: string
  role: string
  kind: 'movie' | 'tvshow'
}

export default function PersonDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [person, setPerson] = useState<Person | null>(null)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef<HTMLDivElement>(null)

  const scrollToTop = () =>
    pageRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  const scrollToBottom = () =>
    pageRef.current?.scrollTo({ top: pageRef.current.scrollHeight, behavior: 'smooth' })

  useEffect(() => {
    if (!id) return
    let alive = true
    api.getPerson(id)
      .then(p => { if (alive) setPerson(p) })
      .catch(e => { if (alive) setError(String(e?.message ?? e)) })
    return () => { alive = false }
  }, [id])

  // Cast + crew rows for the same movie collapse into a single card
  // labeled with both roles ("Director · Writer"), matching the web
  // app's mergeMedia behaviour.
  const movies = useMemo<MediaEntry[]>(() => mergeMedia(
    person?.movie_cast.map(m => ({ id: m.movie_id, title: m.title, year: m.year, posterPath: m.poster_path, role: m.character, kind: 'movie' as const })) ?? [],
    person?.movie_crew.map(m => ({ id: m.movie_id, title: m.title, year: m.year, posterPath: m.poster_path, role: m.job, kind: 'movie' as const })) ?? [],
  ), [person])

  const shows = useMemo<MediaEntry[]>(() => mergeMedia(
    person?.tv_show_cast.map(s => ({ id: s.tv_show_id, title: s.title, year: s.year, posterPath: s.poster_path, role: s.character, kind: 'tvshow' as const })) ?? [],
    person?.tv_show_crew.map(s => ({ id: s.tv_show_id, title: s.title, year: s.year, posterPath: s.poster_path, role: s.job, kind: 'tvshow' as const })) ?? [],
  ), [person])

  return (
    <FocusProvider
      backFocusesTag="sidebar"
      onBack={() => navigate(-1)}
    >
      <div ref={pageRef} style={styles.page}>
        <Sidebar />

        {error && <div style={styles.error}>{error}</div>}

        {person && (
          <main style={styles.main}>
            <BackButton onSelect={() => navigate(-1)} onFocus={scrollToTop} />

            <header style={styles.header}>
              <div style={styles.photo}>
                {imageUrl(person.profile_path) ? (
                  <img src={imageUrl(person.profile_path)} alt={person.name} decoding="async" style={styles.photoImg} />
                ) : (
                  <div style={styles.photoFallback}>{initials(person.name)}</div>
                )}
              </div>
              <div style={styles.headerInfo}>
                <h1 style={styles.name}>{person.name}</h1>
                {person.biography && (
                  <p style={styles.bio}>{person.biography}</p>
                )}
              </div>
            </header>

            {movies.length > 0 && (
              <Section
                title="Movies"
                items={movies}
                navigate={navigate}
                onLastRowFocus={shows.length === 0 ? scrollToBottom : undefined}
              />
            )}

            {shows.length > 0 && (
              <Section
                title="TV Shows"
                items={shows}
                navigate={navigate}
                onLastRowFocus={scrollToBottom}
              />
            )}

            {movies.length === 0 && shows.length === 0 && (
              <p style={styles.empty}>No on-platform credits for this person yet.</p>
            )}
          </main>
        )}
      </div>
    </FocusProvider>
  )
}

function BackButton({ onSelect, onFocus }: { onSelect: () => void; onFocus?: () => void }) {
  // autoFocus — Back is the first thing the user lands on. Most people
  // arrive here from a cast click and the most common next action is
  // returning to the movie. Left override keeps the sidebar reachable
  // even from this corner of the page.
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    autoFocus: true,
    overrides: { left: 'sidebar' },
    onFocusChange: focused => { if (focused) onFocus?.() },
  })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      onClick={onSelect}
      style={styles.backBtn}
    >
      ← Back
    </button>
  )
}

function Section({
  title, items, navigate, onLastRowFocus,
}: {
  title: string
  items: MediaEntry[]
  navigate: (path: string) => void
  // Fired when focus lands on any card in the section's bottom row.
  // Provided only for the bottom-most section of the page so the page
  // scrolls all the way down on focus.
  onLastRowFocus?: () => void
}) {
  const lastRow = Math.floor((items.length - 1) / COLS)
  return (
    <section style={styles.section}>
      <h2 style={styles.sectionTitle}>{title}</h2>
      <div style={styles.grid}>
        {items.map((m, i) => {
          const row = Math.floor(i / COLS)
          return (
            <PosterCard
              key={`${m.kind}:${m.id}`}
              title={m.title}
              subtitle={m.role || (m.year > 0 ? String(m.year) : undefined)}
              imageSrc={m.posterPath || undefined}
              fill
              overrides={{
                left: (i % COLS) === 0 ? 'sidebar' : undefined,
              }}
              onSelect={() => {
                if (m.kind === 'movie') navigate(`/movies/${m.id}`)
                else navigate(`/tvshows/${m.id}`)
              }}
              onFocus={row === lastRow ? onLastRowFocus : undefined}
            />
          )
        })}
      </div>
    </section>
  )
}

function mergeMedia(cast: MediaEntry[], crew: MediaEntry[]): MediaEntry[] {
  const map = new Map<string, MediaEntry>()
  for (const item of cast) map.set(item.id, item)
  for (const item of crew) {
    const existing = map.get(item.id)
    if (existing && existing.role && item.role && existing.role !== item.role) {
      map.set(item.id, { ...existing, role: `${existing.role} · ${item.role}` })
    } else if (!existing) {
      map.set(item.id, item)
    }
  }
  return Array.from(map.values())
}

function initials(name: string): string {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map(w => w[0]!.toUpperCase()).join('')
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    height: '100vh',
    overflowY: 'auto',
    paddingLeft: 'var(--sidebar-rail)',
    scrollbarWidth: 'none',
  },
  main: {
    padding: 'calc(var(--safe-y) + 1.5rem) var(--safe-x) calc(var(--safe-y) + 1.5rem)',
    display: 'flex',
    flexDirection: 'column',
    gap: '2.5rem',
  },
  backBtn: {
    alignSelf: 'flex-start',
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.75rem 1.5rem',
    fontSize: '1rem',
    fontWeight: 600,
    color: 'var(--text)',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-md)',
  },
  header: {
    display: 'flex',
    gap: '2.5rem',
    alignItems: 'flex-start',
  },
  photo: {
    flex: '0 0 auto',
    width: '12rem',
    aspectRatio: '1 / 1',
    borderRadius: '50%',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
  },
  photoImg: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  photoFallback: {
    width: '100%',
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    fontSize: '3rem',
    fontWeight: 700,
    color: 'var(--text-muted)',
  },
  headerInfo: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
    maxWidth: '60rem',
  },
  name: {
    margin: 0,
    fontSize: '2.5rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
  },
  bio: {
    margin: 0,
    fontSize: '1rem',
    lineHeight: 1.5,
    color: 'var(--text-muted)',
    display: '-webkit-box',
    WebkitLineClamp: 5,
    WebkitBoxOrient: 'vertical',
    overflow: 'hidden',
  },
  section: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
  },
  sectionTitle: {
    margin: 0,
    fontSize: '1.5rem',
    fontWeight: 600,
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: `repeat(${COLS}, minmax(0, 1fr))`,
    gap: '1.5rem 1.25rem',
  },
  empty: {
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  error: {
    margin: 'var(--safe-y) var(--safe-x)',
    padding: '1rem 1.25rem',
    background: 'rgba(255, 180, 171, 0.12)',
    color: 'var(--error)',
    borderRadius: 'var(--radius-md)',
  },
}

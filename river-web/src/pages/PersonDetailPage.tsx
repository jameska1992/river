import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { RiArrowLeftLine, RiUserLine, RiArrowDownSLine, RiArrowUpSLine } from 'react-icons/ri'
import type { Person } from '../api'
import { api } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './PersonDetailPage.module.css'

export function PersonDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [person, setPerson] = useState<Person | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [bioExpanded, setBioExpanded] = useState(false)
  const bioRef = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    if (!id) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets loading state before refetching when the route id changes
    setIsLoading(true)
    api.getPerson(id)
      .then(setPerson)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load person'))
      .finally(() => setIsLoading(false))
  }, [id])

  if (isLoading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className={styles.page}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }

  if (!person) return null

  const hasMovies = person.movie_cast.length > 0 || person.movie_crew.length > 0
  const hasShows = person.tv_show_cast.length > 0 || person.tv_show_crew.length > 0

  // Merge cast + crew per movie into a single entry for display
  const movieEntries = mergeMedia(
    person.movie_cast.map(m => ({ id: m.movie_id, title: m.title, year: m.year, posterPath: m.poster_path, role: m.character, linkTo: `/movie/${m.movie_id}` })),
    person.movie_crew.map(m => ({ id: m.movie_id, title: m.title, year: m.year, posterPath: m.poster_path, role: m.job, linkTo: `/movie/${m.movie_id}` })),
  )

  const showEntries = mergeMedia(
    person.tv_show_cast.map(s => ({ id: s.tv_show_id, title: s.title, year: s.year, posterPath: s.poster_path, role: s.character, linkTo: `/show/${s.tv_show_id}` })),
    person.tv_show_crew.map(s => ({ id: s.tv_show_id, title: s.title, year: s.year, posterPath: s.poster_path, role: s.job, linkTo: `/show/${s.tv_show_id}` })),
  )

  return (
    <div className={styles.page}>
      <div className={`container ${styles.header}`}>
        <button className={`btn btn-icon ${styles.backBtn}`} onClick={() => history.back()} aria-label="Go back">
          <RiArrowLeftLine size={20} />
        </button>

        <div className={styles.profile}>
          <div className={styles.photo}>
            {person.profile_path ? (
              <img src={imageUrl(person.profile_path)} alt={person.name} />
            ) : (
              <RiUserLine size={56} />
            )}
          </div>
          <h1 className={`headline-lg ${styles.name}`}>{person.name}</h1>
        </div>

        {person.biography && (
          <div className={styles.bioWrapper}>
            <p
              ref={bioRef}
              className={`body-md ${styles.bio}`}
              // eslint-disable-next-line react-hooks/refs -- reads measured scrollHeight to animate the bio expand; the ?? fallback covers the first render before the ref attaches
              style={{ maxHeight: bioExpanded ? `${bioRef.current?.scrollHeight ?? 9999}px` : '6.8em' }}
            >
              {person.biography}
            </p>
            <button className={styles.bioToggle} onClick={() => setBioExpanded(v => !v)}>
              {bioExpanded ? <><RiArrowUpSLine size={18} /> Show less</> : <><RiArrowDownSLine size={18} /> Show more</>}
            </button>
          </div>
        )}
      </div>

      {hasMovies && (
        <section className={`container ${styles.section}`}>
          <h2 className={`headline-md ${styles.sectionTitle}`}>Movies</h2>
          <div className={styles.grid}>
            {movieEntries.map(entry => (
              <MediaCard key={entry.id} {...entry} />
            ))}
          </div>
        </section>
      )}

      {hasShows && (
        <section className={`container ${styles.section}`}>
          <h2 className={`headline-md ${styles.sectionTitle}`}>TV Shows</h2>
          <div className={styles.grid}>
            {showEntries.map(entry => (
              <MediaCard key={entry.id} {...entry} />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

// ── Helpers ──────────────────────────────────────────────

interface MediaEntry {
  id: string
  title: string
  year: number
  posterPath: string
  role: string
  linkTo: string
}

function mergeMedia(cast: MediaEntry[], crew: MediaEntry[]): MediaEntry[] {
  const map = new Map<string, MediaEntry>()
  for (const item of cast) {
    map.set(item.id, item)
  }
  for (const item of crew) {
    if (map.has(item.id)) {
      const existing = map.get(item.id)!
      if (existing.role && item.role && existing.role !== item.role) {
        map.set(item.id, { ...existing, role: `${existing.role} · ${item.role}` })
      }
    } else {
      map.set(item.id, item)
    }
  }
  return Array.from(map.values())
}

// ── Media card ────────────────────────────────────────────

function MediaCard({ title, year, posterPath, role, linkTo }: MediaEntry) {
  return (
    <Link to={linkTo} className={styles.card}>
      <div className={`card card-portrait ${styles.poster}`}>
        {posterPath ? (
          <img src={imageUrl(posterPath)} alt={title} loading="lazy" />
        ) : (
          <div className={styles.posterFallback} />
        )}
      </div>
      <div className={styles.cardInfo}>
        <span className={`label-md ${styles.cardTitle}`}>{title}</span>
        {year > 0 && <span className={`label-sm ${styles.cardYear}`}>{year}</span>}
        {role && <span className={`label-sm ${styles.cardRole}`}>{role}</span>}
      </div>
    </Link>
  )
}

// ── Skeleton ──────────────────────────────────────────────

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`container ${styles.header}`}>
        <div className={styles.profile}>
          <div className={`${styles.photo} ${styles.skeleton}`} />
          <div className={`${styles.skeleton} ${styles.skeletonTitle}`} />
        </div>
      </div>
      <div className={`container ${styles.section}`}>
        <div className={styles.grid}>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className={styles.card}>
              <div className={`card card-portrait ${styles.poster} ${styles.skeleton}`} />
              <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

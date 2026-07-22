import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { RiFilmLine, RiTv2Line, RiHeadphoneLine, RiUserLine } from 'react-icons/ri'
import type { SearchResult, SearchResultItem, PersonSearchResult } from '../api'
import { api } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './SearchPage.module.css'

export function SearchPage() {
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const genre = searchParams.get('genre') ?? ''

  const [result, setResult] = useState<SearchResult | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!query.trim() && !genre.trim()) {
      setResult(null)
      return
    }
    setIsLoading(true)
    setError(null)
    api.search({ q: query || undefined, genre: genre || undefined })
      .then(setResult)
      .catch(err => setError(err instanceof Error ? err.message : 'Search failed'))
      .finally(() => setIsLoading(false))
  }, [query, genre])

  const totalItems = result
    ? result.libraries.reduce((n, l) => n + l.items.length, 0) + result.people.length
    : 0

  const headingText = isLoading ? 'Searching…'
    : genre && query ? `"${query}" in ${genre}`
    : genre          ? genre
    : query          ? `"${query}"`
    : 'Search'

  return (
    <div className={`container ${styles.page}`}>
      <div className={styles.heading}>
        <h1 className="headline-md">{headingText}</h1>
        {!isLoading && result && (
          <span className={`label-sm ${styles.count}`}>
            {totalItems === 0 ? 'No results' : `${totalItems} result${totalItems !== 1 ? 's' : ''}`}
          </span>
        )}
      </div>

      {error && <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>}

      {isLoading && <LoadingSkeleton />}

      {!isLoading && result && totalItems === 0 && (
        <div className={styles.empty}>
          <p className="body-md">Nothing matched your search. Try a different term.</p>
        </div>
      )}

      {!isLoading && result && (
        <>
          {result.libraries.map(lib => (
            <section key={lib.library_id} className={styles.section}>
              <h2 className={`headline-sm ${styles.sectionTitle}`}>
                <LibraryIcon type={lib.library_type} />
                {lib.library_name}
              </h2>
              <div className={styles.grid}>
                {lib.items.map(item => (
                  <MediaCard key={item.id} item={item} />
                ))}
              </div>
            </section>
          ))}

          {result.people.length > 0 && (
            <section className={styles.section}>
              <h2 className={`headline-sm ${styles.sectionTitle}`}>
                <RiUserLine size={18} />
                People
              </h2>
              <div className={styles.peopleGrid}>
                {result.people.map(p => (
                  <PersonCard key={p.id} person={p} />
                ))}
              </div>
            </section>
          )}
        </>
      )}
    </div>
  )
}

// ── Media card ────────────────────────────────────────────

function MediaCard({ item }: { item: SearchResultItem }) {
  const to = item.media_type === 'movie'
    ? `/movie/${item.id}`
    : item.media_type === 'tvshow'
    ? `/show/${item.id}`
    : `/audiobook/${item.id}`
  return (
    <Link to={to} className={styles.mediaCard}>
      <div className={`card card-portrait ${styles.poster}`}>
        {item.poster_path ? (
          <img src={imageUrl(item.poster_path)} alt={item.title} loading="lazy" />
        ) : (
          <div className={styles.posterFallback}>
            {item.media_type === 'movie' ? <RiFilmLine size={32} /> : item.media_type === 'tvshow' ? <RiTv2Line size={32} /> : <RiHeadphoneLine size={32} />}
          </div>
        )}
      </div>
      <div className={styles.cardMeta}>
        <span className={`label-md ${styles.cardTitle}`}>{item.title}</span>
        {item.year > 0 && <span className={`label-sm ${styles.cardYear}`}>{item.year}</span>}
      </div>
    </Link>
  )
}

// ── Person card ───────────────────────────────────────────

function PersonCard({ person }: { person: PersonSearchResult }) {
  return (
    <Link to={`/person/${person.id}`} className={styles.personCard}>
      <div className={styles.personPhoto}>
        {person.profile_path ? (
          <img src={imageUrl(person.profile_path)} alt={person.name} />
        ) : (
          <RiUserLine size={28} />
        )}
      </div>
      <span className={`label-sm ${styles.personName}`}>{person.name}</span>
    </Link>
  )
}

// ── Library icon ──────────────────────────────────────────

function LibraryIcon({ type }: { type: string }) {
  if (type === 'movie') return <RiFilmLine size={18} />
  if (type === 'tvshow') return <RiTv2Line size={18} />
  if (type === 'audiobook') return <RiHeadphoneLine size={18} />
  return null
}

// ── Skeleton ──────────────────────────────────────────────

function LoadingSkeleton() {
  return (
    <div className={styles.section}>
      <div className={`${styles.skeleton} ${styles.skeletonHeading}`} />
      <div className={styles.grid}>
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className={styles.mediaCard}>
            <div className={`card card-portrait ${styles.poster} ${styles.skeleton}`} />
            <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
          </div>
        ))}
      </div>
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiArrowLeftSLine, RiArrowRightSLine, RiFilmLine, RiHeadphoneLine, RiTv2Line } from 'react-icons/ri'
import { api } from '../api'
import type { SimilarItem } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './SimilarCarousel.module.css'

// SimilarCarousel renders the "More like this" row on movie / show /
// audiobook detail pages. Fetches on mount, renders nothing when the
// server returns an empty list (source has no genres, or nothing in
// the library shares a genre) — better to omit the row than show an
// empty heading.
//
// Reuses the ContinueWatching CSS module so section spacing / card
// sizing stay visually consistent with the existing home-page rails.
export function SimilarCarousel({
  sourceId,
  type,
}: {
  sourceId: string
  type: 'movie' | 'tvshow' | 'audiobook'
}) {
  const [items, setItems] = useState<SimilarItem[]>([])
  const [loaded, setLoaded] = useState(false)
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  useEffect(() => {
    let alive = true
    setLoaded(false)
    setItems([])
    const fetcher =
      type === 'movie' ? api.getSimilarMovies
        : type === 'tvshow' ? api.getSimilarShows
          : api.getSimilarAudiobooks
    fetcher.call(api, sourceId)
      .then(data => { if (alive) { setItems(data); setLoaded(true) } })
      .catch(() => { if (alive) setLoaded(true) })
    return () => { alive = false }
  }, [sourceId, type])

  const syncArrows = () => {
    const el = trackRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4)
  }

  // Re-evaluate arrow visibility once the items render and whenever the track
  // resizes (viewport / breakpoint changes).
  useEffect(() => {
    syncArrows()
    const el = trackRef.current
    if (!el) return
    const ro = new ResizeObserver(syncArrows)
    ro.observe(el)
    return () => ro.disconnect()
  }, [items])

  const scroll = (dir: -1 | 1) => {
    const el = trackRef.current
    if (!el) return
    el.scrollBy({ left: el.clientWidth * 0.8 * dir, behavior: 'smooth' })
  }

  if (!loaded || items.length === 0) return null

  return (
    <section className={styles.section}>
      <h2 className={`headline-sm ${styles.heading}`}>More like this</h2>
      <div className={styles.carousel}>
        {canLeft && <div className={`${styles.fade} ${styles.fadeLeft}`} />}
        {canRight && <div className={`${styles.fade} ${styles.fadeRight}`} />}
        {canLeft && (
          <button className={`${styles.arrow} ${styles.arrowLeft}`} onClick={() => scroll(-1)} aria-label="Scroll left">
            <RiArrowLeftSLine size={26} />
          </button>
        )}
        {canRight && (
          <button className={`${styles.arrow} ${styles.arrowRight}`} onClick={() => scroll(1)} aria-label="Scroll right">
            <RiArrowRightSLine size={26} />
          </button>
        )}
        <div ref={trackRef} className={styles.row} onScroll={syncArrows}>
          {items.map(item => (
            <SimilarCard key={`${item.type}:${item.id}`} item={item} />
          ))}
        </div>
      </div>
    </section>
  )
}

function SimilarCard({ item }: { item: SimilarItem }) {
  const to =
    item.type === 'movie' ? `/movie/${item.id}`
      : item.type === 'tvshow' ? `/show/${item.id}`
        : `/audiobook/${item.id}`

  const fallbackIcon =
    item.type === 'movie' ? <RiFilmLine size={32} />
      : item.type === 'tvshow' ? <RiTv2Line size={32} />
        : <RiHeadphoneLine size={32} />

  // Prefer landscape backdrop art when we have it (movies + shows).
  // Audiobooks fall back to their cover — square-ish, but the card
  // frame is landscape so it'll pillarbox nicely.
  const imageSrc = item.backdrop_path || item.poster_path

  return (
    <Link to={to} className={styles.card}>
      <div className={styles.thumb}>
        {imageSrc ? (
          <img
            src={imageUrl(imageSrc, item.backdrop_path ? 'backdrop' : 'poster')}
            alt={item.title}
            loading="lazy"
            className={styles.poster}
          />
        ) : (
          <div className={styles.fallback}>{fallbackIcon}</div>
        )}
        <div className={styles.overlay} />
      </div>
      <div className={styles.info}>
        <p className={`label-md ${styles.title}`}>{item.title}</p>
        {item.year != null && item.year > 0 && (
          <p className={`label-sm ${styles.showTitle}`}>{item.year}</p>
        )}
      </div>
    </Link>
  )
}

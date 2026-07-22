import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  RiArrowRightLine, RiArrowLeftSLine, RiArrowRightSLine,
  RiFilmLine, RiTv2Line, RiHeadphoneLine,
} from 'react-icons/ri'
import { useWatchlist } from '../context/WatchlistContext'
import { api } from '../api'
import type { WatchlistItem } from '../api'
import { MediaCard } from './MediaCard'
import styles from './LibraryCarousels.module.css'

const FALLBACK_ICONS: Record<WatchlistItem['media_type'], React.ReactNode> = {
  movie:     <RiFilmLine />,
  tvshow:    <RiTv2Line />,
  audiobook: <RiHeadphoneLine />,
}

const DETAIL_PATHS: Record<WatchlistItem['media_type'], (id: string) => string> = {
  movie:     id => `/movie/${id}`,
  tvshow:    id => `/show/${id}`,
  audiobook: id => `/audiobook/${id}`,
}

export function WatchlistCarousel() {
  const { items, isInWatchlist, toggle } = useWatchlist()

  if (items.length === 0) return null

  return (
    <section className={styles.row}>
      <div className={styles.rowHeader}>
        <Link to="/watchlist" className={styles.rowTitleLink}>
          <span className={`headline-sm ${styles.rowTitle}`}>Watchlist</span>
          <RiArrowRightLine size={18} className={styles.rowTitleArrow} />
        </Link>
      </div>
      <WatchlistTrack items={items} isInWatchlist={isInWatchlist} toggle={toggle} />
    </section>
  )
}

function WatchlistTrack({
  items,
  isInWatchlist,
  toggle,
}: {
  items: WatchlistItem[]
  isInWatchlist: (type: string, id: string) => boolean
  toggle: (type: WatchlistItem['media_type'], id: string) => void
}) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)
  const navigate = useNavigate()
  const [loadingPlayId, setLoadingPlayId] = useState<string | null>(null)

  const syncArrows = () => {
    const el = trackRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4)
  }

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

  const playItem = async (item: WatchlistItem) => {
    setLoadingPlayId(item.media_id)
    try {
      if (item.media_type === 'movie') {
        navigate(`/movie/${item.media_id}/watch`)
      } else if (item.media_type === 'tvshow') {
        const { season_id, episode_id } = await api.getNextEpisode(item.media_id)
        navigate(`/show/${item.media_id}/season/${season_id}/episode/${episode_id}/watch`)
      } else {
        navigate(`/audiobook/${item.media_id}/listen`)
      }
    } catch {
      navigate(DETAIL_PATHS[item.media_type](item.media_id))
    } finally {
      setLoadingPlayId(null)
    }
  }

  return (
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
      <div ref={trackRef} className={styles.track} onScroll={syncArrows}>
        {items.map(item => (
          <div key={item.id} className={styles.item}>
            <MediaCard
              title={item.title}
              subtitle={item.year > 0 ? String(item.year) : undefined}
              imageSrc={item.poster_path || undefined}
              fallbackIcon={FALLBACK_ICONS[item.media_type]}
              to={DETAIL_PATHS[item.media_type](item.media_id)}
              onPlay={item.media_type !== 'audiobook' ? () => playItem(item) : undefined}
              playLoading={loadingPlayId === item.media_id}
              inWatchlist={isInWatchlist(item.media_type, item.media_id)}
              onWatchlistToggle={() => toggle(item.media_type, item.media_id)}
            />
          </div>
        ))}
      </div>
    </div>
  )
}

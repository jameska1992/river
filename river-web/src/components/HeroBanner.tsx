import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiPlayFill, RiInformationLine, RiArrowLeftSLine, RiArrowRightSLine, RiStarLine } from 'react-icons/ri'
import { api } from '../api'
import type { RecentlyAddedItem, ContinueWatchingItem } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './HeroBanner.module.css'

const SLIDE_DURATION = 7000

export function HeroBanner() {
  const [items, setItems] = useState<RecentlyAddedItem[]>([])
  const [continueWatching, setContinueWatching] = useState<ContinueWatchingItem[]>([])
  const [activeIndex, setActiveIndex] = useState(0)
  const [loaded, setLoaded] = useState(false)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pausedRef = useRef(false)

  useEffect(() => {
    Promise.all([
      api.getRecentlyAdded(),
      api.getContinueWatching().catch(() => [] as ContinueWatchingItem[]),
    ])
      .then(([data, cw]) => { setItems(data); setContinueWatching(cw); setLoaded(true) })
      .catch(() => setLoaded(true))
  }, [])

  useEffect(() => {
    if (items.length < 2) return
    timerRef.current = setInterval(() => {
      if (!pausedRef.current) setActiveIndex(i => (i + 1) % items.length)
    }, SLIDE_DURATION)
    return () => { if (timerRef.current) clearInterval(timerRef.current) }
  }, [items.length])

  if (!loaded || items.length === 0) {
    return loaded ? null : <div className={styles.skeleton} />
  }

  const prev = () => setActiveIndex(i => (i - 1 + items.length) % items.length)
  const next = () => setActiveIndex(i => (i + 1) % items.length)

  return (
    <div
      className={styles.hero}
      onMouseEnter={() => { pausedRef.current = true }}
      onMouseLeave={() => { pausedRef.current = false }}
    >
      {items.map((item, i) => (
        <Slide key={item.id} item={item} active={i === activeIndex} continueWatching={continueWatching} />
      ))}

      {items.length > 1 && (
        <>
          <button className={`${styles.arrow} ${styles.arrowLeft}`} onClick={prev} aria-label="Previous">
            <RiArrowLeftSLine size={28} />
          </button>
          <button className={`${styles.arrow} ${styles.arrowRight}`} onClick={next} aria-label="Next">
            <RiArrowRightSLine size={28} />
          </button>
        </>
      )}

      {items.length > 1 && (
        <div className={styles.dots}>
          {items.map((item, i) => (
            <button
              key={item.id}
              className={`${styles.dot} ${i === activeIndex ? styles.dotActive : ''}`}
              onClick={() => setActiveIndex(i)}
              aria-label={`Go to slide ${i + 1}`}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ── Slide ─────────────────────────────────────────────────

type CTALabel = 'Continue' | 'Play next' | 'Play first episode'

interface TVShowCTA {
  label: CTALabel
  watchUrl: string | null
  resolved: boolean
}

function Slide({
  item,
  active,
  continueWatching,
}: {
  item: RecentlyAddedItem
  active: boolean
  continueWatching: ContinueWatchingItem[]
}) {
  const detailPath = item.media_type === 'movie' ? `/movie/${item.id}` : `/show/${item.id}`
  const movieWatchUrl = item.media_type === 'movie' && item.file_path ? `/movie/${item.id}/watch` : null

  const [tvCTA, setTvCTA] = useState<TVShowCTA>({ label: 'Play first episode', watchUrl: null, resolved: false })
  const resolvedRef = useRef(false)

  useEffect(() => {
    if (item.media_type !== 'tvshow' || resolvedRef.current || !active) return
    resolvedRef.current = true

    const cwItem = continueWatching.find(cw => cw.show_id === item.id)

    if (cwItem && !cwItem.completed && cwItem.season_id) {
      setTvCTA({
        label: 'Continue',
        watchUrl: `/show/${item.id}/season/${cwItem.season_id}/episode/${cwItem.media_id}/watch`,
        resolved: true,
      })
      return
    }

    resolveTVShowCTA(item.id, cwItem ?? null)
      .then(cta => setTvCTA({ ...cta, resolved: true }))
      .catch(() => setTvCTA(s => ({ ...s, resolved: true })))
  }, [active, item.id, item.media_type, continueWatching])

  const description = item.description.length > 200
    ? item.description.slice(0, 200).trimEnd() + '…'
    : item.description

  const isTVShow = item.media_type === 'tvshow'
  const primaryWatchUrl = isTVShow ? tvCTA.watchUrl : movieWatchUrl
  const primaryLabel: string = isTVShow ? tvCTA.label : 'Play'
  const primaryReady = isTVShow ? tvCTA.resolved : true

  return (
    <div className={`${styles.slide} ${active ? styles.slideActive : ''}`} aria-hidden={!active}>
      {item.backdrop_path ? (
        <img src={imageUrl(item.backdrop_path, 'backdrop')} alt="" className={styles.backdrop} loading="eager" />
      ) : (
        <div className={styles.backdropFallback} />
      )}

      <div className={styles.gradientTop} />
      <div className={styles.gradientBottom} />

      <div className={`container ${styles.content}`}>
        <div className={styles.meta}>
          {item.genres.length > 0 && (
            <div className={styles.genres}>
              {item.genres.slice(0, 3).map(g => (
                <span key={g} className="badge badge-primary">{g}</span>
              ))}
            </div>
          )}

          <h1 className={`display-lg ${styles.title}`}>{item.title}</h1>

          <div className={styles.attrs}>
            {item.year > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {item.year}
              </span>
            )}
            {item.rating > 0 && (
              <span className={styles.rating}>
                <RiStarLine size={14} />
                <span className="label-md">{item.rating.toFixed(1)}</span>
              </span>
            )}
            <span className="badge badge-secondary">
              {item.media_type === 'movie' ? 'Movie' : 'Series'}
            </span>
          </div>

          {description && (
            <p className={`body-md ${styles.description}`}>{description}</p>
          )}

          <div className={styles.actions}>
            {primaryWatchUrl ? (
              <Link to={primaryWatchUrl} className="btn btn-primary btn-lg">
                <RiPlayFill size={20} />
                {primaryLabel}
              </Link>
            ) : (primaryReady && !isTVShow) ? null : (
              <button className="btn btn-primary btn-lg" disabled>
                <RiPlayFill size={20} />
                {primaryLabel}
              </button>
            )}
            <Link to={detailPath} className="btn btn-secondary btn-lg">
              <RiInformationLine size={20} />
              More Info
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}

// ── TV show CTA resolution ────────────────────────────────

async function resolveTVShowCTA(
  showId: string,
  cwItem: ContinueWatchingItem | null,
): Promise<Omit<TVShowCTA, 'resolved'>> {
  const seasons = await api.listSeasons(showId)
  const sorted = [...seasons].sort((a, b) => a.number - b.number)
  if (sorted.length === 0) return { label: 'Play first episode', watchUrl: null }

  if (cwItem?.completed && cwItem.season_number != null && cwItem.episode_number != null) {
    // Try next episode in the same season
    const curSeason = sorted.find(s => s.number === cwItem.season_number)
    if (curSeason) {
      const eps = await api.listEpisodes(showId, curSeason.id)
      const next = [...eps]
        .sort((a, b) => a.number - b.number)
        .find(e => e.number > cwItem.episode_number! && e.file_path)
      if (next) return { label: 'Play next', watchUrl: `/show/${showId}/season/${curSeason.id}/episode/${next.id}/watch` }
    }
    // Try first episode of the next season
    const curIdx = sorted.findIndex(s => s.number === cwItem.season_number)
    if (curIdx >= 0 && curIdx + 1 < sorted.length) {
      const nextSeason = sorted[curIdx + 1]
      const eps = await api.listEpisodes(showId, nextSeason.id)
      const first = [...eps].sort((a, b) => a.number - b.number).find(e => e.file_path)
      if (first) return { label: 'Play next', watchUrl: `/show/${showId}/season/${nextSeason.id}/episode/${first.id}/watch` }
    }
  }

  // Fall back to first available episode
  for (const season of sorted) {
    const eps = await api.listEpisodes(showId, season.id)
    const first = [...eps].sort((a, b) => a.number - b.number).find(e => e.file_path)
    if (first) return { label: 'Play first episode', watchUrl: `/show/${showId}/season/${season.id}/episode/${first.id}/watch` }
  }

  return { label: 'Play first episode', watchUrl: null }
}

import { Link } from 'react-router-dom'
import type { ReactNode } from 'react'
import { RiPlayFill, RiCheckLine, RiBookmarkLine, RiBookmarkFill, RiAlertFill } from 'react-icons/ri'
import { imageUrl } from '../util/imageUrl'
import styles from './MediaCard.module.css'

interface MediaCardProps {
  title: string
  subtitle?: string
  rating?: string
  imageSrc?: string
  fallbackIcon: ReactNode
  badge?: string
  to?: string
  aspect?: 'portrait' | 'landscape'
  onPlay?: () => void
  playLoading?: boolean
  // playNotReady shows a yellow warning glyph beside the play button when
  // the canonical post-transcode file isn't on disk yet. The play action
  // still fires — river-api falls back to streaming the original source —
  // and the warning is just a hint that quality/format may not be ideal.
  playNotReady?: boolean
  progressRatio?: number
  inWatchlist?: boolean
  onWatchlistToggle?: () => void
  // Explicit watched flag. When undefined, the badge falls back to the
  // legacy inference (progressRatio >= 0.9 implies watched) so existing
  // callers that only pass progressRatio keep their behavior.
  completed?: boolean
  // When provided, the watched badge is always rendered (active when
  // watched, hollow when not) and clicking it calls this in either
  // direction. Without this prop, the badge only appears in the active
  // state and is not interactive.
  onCompletedToggle?: () => void
}

export function MediaCard({
  title, subtitle, rating, imageSrc, fallbackIcon, badge, to, aspect = 'portrait', onPlay, playLoading, playNotReady, progressRatio,
  inWatchlist, onWatchlistToggle, completed, onCompletedToggle,
}: MediaCardProps) {
  const hasImage = !!imageSrc

  const isCompleted = completed === true
    || (completed === undefined && progressRatio !== undefined && progressRatio >= 0.9)
  const showWatchedBadge = isCompleted || onCompletedToggle !== undefined
  const showProgressBar = !isCompleted && progressRatio !== undefined && progressRatio > 0

  const handlePlay = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onPlay?.()
  }

  const inner = (
    <div className={`card ${aspect === 'portrait' ? 'card-portrait' : 'card-landscape'} ${styles.card}`}>
      {hasImage ? (
        <img src={imageUrl(imageSrc, aspect === 'landscape' ? 'backdrop' : 'poster')} alt={title} loading="lazy" />
      ) : (
        <div className={styles.placeholder}>
          <span className={styles.placeholderIcon}>{fallbackIcon}</span>
        </div>
      )}

      <div className="card-overlay" />

      {onWatchlistToggle !== undefined && (
        <button
          className={`${styles.watchlistBtn} ${inWatchlist ? styles.watchlistBtnActive : ''}`}
          onClick={e => { e.preventDefault(); e.stopPropagation(); onWatchlistToggle() }}
          aria-label={inWatchlist ? 'Remove from watchlist' : 'Add to watchlist'}
        >
          {inWatchlist ? <RiBookmarkFill size={14} /> : <RiBookmarkLine size={14} />}
        </button>
      )}

      {showWatchedBadge && (
        onCompletedToggle ? (
          <button
            className={`${styles.completedBadge} ${styles.completedBadgeBtn} ${isCompleted ? '' : styles.completedBadgeInactive}`}
            onClick={e => { e.preventDefault(); e.stopPropagation(); onCompletedToggle() }}
            aria-label={isCompleted ? 'Mark as unwatched' : 'Mark as watched'}
            aria-pressed={isCompleted}
            title={isCompleted ? 'Mark as unwatched' : 'Mark as watched'}
          >
            <RiCheckLine size={14} />
          </button>
        ) : (
          <div className={styles.completedBadge}>
            <RiCheckLine size={14} />
          </div>
        )
      )}
      {showProgressBar && (
        <div className={styles.progressBar}>
          <div className={styles.progressFill} style={{ width: `${progressRatio! * 100}%` }} />
        </div>
      )}

      {onPlay && (
        <div className={styles.playOverlay}>
          <div className={styles.playWrap}>
            <button
              className={styles.playBtn}
              onClick={handlePlay}
              aria-label={`Play ${title}`}
            >
              {playLoading ? (
                <span className={styles.spinner} />
              ) : (
                <RiPlayFill size={20} />
              )}
            </button>
            {playNotReady && !playLoading && (
              <span
                className={styles.notReadyBadge}
                title="Not transcoded yet — playing from original source"
                aria-label="Not transcoded yet — playing from original source"
              >
                <RiAlertFill size={14} />
              </span>
            )}
          </div>
        </div>
      )}

      <div className={styles.meta}>
        {badge && <span className={`badge badge-primary ${styles.badge}`}>{badge}</span>}
        <p className={styles.title}>{title}</p>
        {(subtitle || rating) && (
          <div className={styles.subtitleRow}>
            {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
            {rating && <span className={styles.rating}>{rating}</span>}
          </div>
        )}
      </div>
    </div>
  )

  if (to) {
    return <Link to={to} className={styles.link}>{inner}</Link>
  }
  return inner
}

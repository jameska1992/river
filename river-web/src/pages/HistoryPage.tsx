import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  RiFilmLine, RiTv2Line, RiHeadphoneLine, RiCloseLine,
  RiHistoryLine, RiEyeLine, RiEyeOffLine,
} from 'react-icons/ri'
import { api } from '../api'
import type { ContinueWatchingItem } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './HistoryPage.module.css'

// Buckets group the (already recency-ordered) history under friendly date
// headings. Order here is the render order.
const BUCKETS = ['Today', 'This week', 'This month', 'Earlier'] as const
type Bucket = (typeof BUCKETS)[number]

function bucketOf(iso: string, now: Date): Bucket {
  const d = new Date(iso)
  if (
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  ) return 'Today'
  const days = (now.getTime() - d.getTime()) / 86_400_000
  if (days < 7) return 'This week'
  if (days < 30) return 'This month'
  return 'Earlier'
}

export function HistoryPage() {
  const [items, setItems] = useState<ContinueWatchingItem[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    api.getHistory()
      .then(data => { setItems(data); setLoaded(true) })
      .catch(() => setLoaded(true))
  }, [])

  const remove = (item: ContinueWatchingItem) => {
    api.deleteProgress(item.media_type, item.media_id).catch(() => {})
    setItems(prev => prev.filter(i => i.media_id !== item.media_id))
  }

  const toggleWatched = (item: ContinueWatchingItem) => {
    const next = !item.completed
    api.setProgressCompleted(item.media_type, item.media_id, next).catch(() => {})
    setItems(prev => prev.map(i =>
      i.media_id === item.media_id ? { ...i, completed: next } : i,
    ))
  }

  if (!loaded) return null

  // Partition into date buckets, preserving the server's recency order.
  const now = new Date()
  const groups = new Map<Bucket, ContinueWatchingItem[]>()
  for (const item of items) {
    const b = bucketOf(item.updated_at, now)
    const arr = groups.get(b) ?? []
    arr.push(item)
    groups.set(b, arr)
  }

  return (
    <div className={`container ${styles.page}`}>
      <div className={styles.heading}>
        <h1 className="headline-lg">Watch History</h1>
        {items.length > 0 && (
          <span className={`label-sm ${styles.count}`}>
            {items.length} {items.length === 1 ? 'item' : 'items'}
          </span>
        )}
      </div>

      {items.length === 0 ? (
        <div className={styles.empty}>
          <RiHistoryLine size={40} className={styles.emptyIcon} />
          <p className="body-md">No watch history yet.</p>
          <p className="body-sm">
            Movies, shows, and audiobooks you play will show up here so you can pick up where you left off.
          </p>
        </div>
      ) : (
        BUCKETS.filter(b => groups.has(b)).map(b => (
          <section key={b} className={styles.group}>
            <h2 className={`label-lg ${styles.groupHeading}`}>{b}</h2>
            <div className={styles.grid}>
              {groups.get(b)!.map(item => (
                <HistoryCard
                  key={item.media_id}
                  item={item}
                  onRemove={remove}
                  onToggleWatched={toggleWatched}
                />
              ))}
            </div>
          </section>
        ))
      )}
    </div>
  )
}

function HistoryCard({
  item,
  onRemove,
  onToggleWatched,
}: {
  item: ContinueWatchingItem
  onRemove: (item: ContinueWatchingItem) => void
  onToggleWatched: (item: ContinueWatchingItem) => void
}) {
  // Route by media type (mirrors the Continue Watching rail). Audiobook
  // chapters resume via the ?chapter query param the listen page handles.
  const watchPath =
    item.media_type === 'movie'
      ? `/movie/${item.media_id}/watch`
      : item.media_type === 'chapter' && item.audiobook_id
        ? `/audiobook/${item.audiobook_id}/listen?chapter=${item.media_id}`
        : `/show/${item.show_id}/season/${item.season_id}/episode/${item.media_id}/watch`

  const pct = item.duration > 0 ? Math.min(item.position / item.duration, 1) * 100 : 0

  const epLabel = item.media_type === 'episode' && item.season_number != null && item.episode_number != null
    ? `S${String(item.season_number).padStart(2, '0')}E${String(item.episode_number).padStart(2, '0')}`
    : null

  const chapterLabel = item.media_type === 'chapter' && item.chapter_number != null
    ? `Chapter ${item.chapter_number}${item.chapter_title ? ` · ${item.chapter_title}` : ''}`
    : null

  const fallbackIcon = item.media_type === 'movie'
    ? <RiFilmLine size={32} />
    : item.media_type === 'chapter'
      ? <RiHeadphoneLine size={32} />
      : <RiTv2Line size={32} />

  const imageSrc = item.backdrop_path || item.poster_path

  const stop = (fn: () => void) => (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    fn()
  }

  return (
    <Link to={watchPath} className={styles.card}>
      <div className={styles.thumb}>
        {imageSrc ? (
          <img src={imageUrl(imageSrc, 'backdrop')} alt={item.title} loading="lazy" className={styles.poster} />
        ) : (
          <div className={styles.fallback}>{fallbackIcon}</div>
        )}
        <div className={styles.overlay} />
        {item.completed && <span className={styles.watchedBadge}>Watched</span>}
        <div className={styles.progressBar}>
          <div className={styles.progressFill} style={{ width: `${item.completed ? 100 : pct}%` }} />
        </div>
        <div className={styles.actions}>
          <button
            className={styles.action}
            onClick={stop(() => onToggleWatched(item))}
            aria-label={item.completed ? 'Mark as unwatched' : 'Mark as watched'}
            title={item.completed ? 'Mark as unwatched' : 'Mark as watched'}
          >
            {item.completed ? <RiEyeOffLine size={14} /> : <RiEyeLine size={14} />}
          </button>
          <button
            className={styles.action}
            onClick={stop(() => onRemove(item))}
            aria-label="Remove from history"
            title="Remove from history"
          >
            <RiCloseLine size={14} />
          </button>
        </div>
      </div>
      <div className={styles.info}>
        {item.show_title && (
          <p className={`label-sm ${styles.showTitle}`}>{item.show_title}</p>
        )}
        <p className={`label-md ${styles.title}`}>
          {epLabel && <span className={styles.epLabel}>{epLabel} · </span>}
          {item.title}
        </p>
        {chapterLabel && (
          <p className={`label-sm ${styles.showTitle}`}>{chapterLabel}</p>
        )}
      </div>
    </Link>
  )
}

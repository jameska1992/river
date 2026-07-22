import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiFilmLine, RiTv2Line, RiHeadphoneLine, RiCloseLine } from 'react-icons/ri'
import { api } from '../api'
import type { ContinueWatchingItem } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './ContinueWatching.module.css'

export function ContinueWatching() {
  const [items, setItems] = useState<ContinueWatchingItem[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    api.getContinueWatching()
      .then(data => { setItems(data); setLoaded(true) })
      .catch(() => setLoaded(true))
  }, [])

  if (!loaded || items.length === 0) return null

  const dismiss = (item: ContinueWatchingItem, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    api.deleteProgress(item.media_type, item.media_id).catch(() => {})
    setItems(prev => prev.filter(i => i.media_id !== item.media_id))
  }

  return (
    <section className={styles.section}>
      <h2 className={`headline-sm ${styles.heading}`}>Continue Watching</h2>
      <div className={styles.row}>
        {items.map(item => (
          <ContinueCard key={item.media_id} item={item} onDismiss={dismiss} />
        ))}
      </div>
    </section>
  )
}

function ContinueCard({
  item,
  onDismiss,
}: {
  item: ContinueWatchingItem
  onDismiss: (item: ContinueWatchingItem, e: React.MouseEvent) => void
}) {
  // Route by media type. Audiobook chapters jump straight into the listen
  // page with the chapter pre-selected via the existing ?chapter query
  // param (AudiobookListenPage already handles it).
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

  // For audiobooks the API puts the audiobook title in `title` and the
  // chapter number/title in dedicated fields, so we render
  // "Audiobook — Chapter N: Title".
  const chapterLabel = item.media_type === 'chapter' && item.chapter_number != null
    ? `Chapter ${item.chapter_number}${item.chapter_title ? ` · ${item.chapter_title}` : ''}`
    : null

  const fallbackIcon = item.media_type === 'movie'
    ? <RiFilmLine size={32} />
    : item.media_type === 'chapter'
      ? <RiHeadphoneLine size={32} />
      : <RiTv2Line size={32} />

  // Prefer the wide cover/backdrop for movies and episodes so the
  // landscape card frames naturally; fall back to whatever was sent (the
  // square audiobook cover, or a portrait poster when no backdrop has
  // been ingested yet).
  const imageSrc = item.backdrop_path || item.poster_path

  return (
    <Link to={watchPath} className={styles.card}>
      <div className={styles.thumb}>
        {imageSrc ? (
          <img src={imageUrl(imageSrc, 'backdrop')} alt={item.title} loading="lazy" className={styles.poster} />
        ) : (
          <div className={styles.fallback}>{fallbackIcon}</div>
        )}
        <div className={styles.overlay} />
        <div className={styles.progressBar}>
          <div className={styles.progressFill} style={{ width: `${pct}%` }} />
        </div>
        <button
          className={styles.dismiss}
          onClick={e => onDismiss(item, e)}
          aria-label="Remove from continue watching"
        >
          <RiCloseLine size={14} />
        </button>
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

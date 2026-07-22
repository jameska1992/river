import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiCloseLine, RiTv2Line } from 'react-icons/ri'
import { api } from '../api'
import type { NextUpItem } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './ContinueWatching.module.css'

// NextUp renders the home-page "Next Up" rail — one card per TV show
// the user has recently completed an episode of, each pointing at that
// show's next canonical episode. Distinct from Continue Watching:
//   - Continue Watching = resume mid-episode.
//   - Next Up           = you finished one, here's what to start next.
//
// Sits above Continue Watching on the home page. When empty it renders
// nothing (returns null) so it doesn't leave an orphan heading.
export function NextUp() {
  const [items, setItems] = useState<NextUpItem[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    api.getNextUp()
      .then(data => { setItems(data); setLoaded(true) })
      .catch(() => setLoaded(true))
  }, [])

  if (!loaded || items.length === 0) return null

  // Optimistic dismiss: remove the row locally first, then POST. If the
  // dismiss fails silently on the wire, the next fetch will bring it
  // back — better UX than blocking on the round-trip.
  const dismiss = (item: NextUpItem, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setItems(prev => prev.filter(i => i.media_id !== item.media_id))
    api.dismissNextUp(item.media_id).catch(() => {})
  }

  return (
    <section className={styles.section}>
      <h2 className={`headline-sm ${styles.heading}`}>Next Up</h2>
      <div className={styles.row}>
        {items.map(item => (
          <NextUpCard key={item.media_id} item={item} onDismiss={dismiss} />
        ))}
      </div>
    </section>
  )
}

function NextUpCard({
  item,
  onDismiss,
}: {
  item: NextUpItem
  onDismiss: (item: NextUpItem, e: React.MouseEvent) => void
}) {
  // Jump straight into the player. Same shape as Continue Watching's
  // episode route.
  const watchPath = `/show/${item.show_id}/season/${item.season_id}/episode/${item.media_id}/watch`

  const epLabel = `S${String(item.season_number).padStart(2, '0')}E${String(item.episode_number).padStart(2, '0')}`

  const imageSrc = item.backdrop_path || item.poster_path

  return (
    <Link to={watchPath} className={styles.card}>
      <div className={styles.thumb}>
        {imageSrc ? (
          <img src={imageUrl(imageSrc, 'backdrop')} alt={item.title} loading="lazy" className={styles.poster} />
        ) : (
          <div className={styles.fallback}><RiTv2Line size={32} /></div>
        )}
        <div className={styles.overlay} />
        <button
          className={styles.dismiss}
          onClick={e => onDismiss(item, e)}
          aria-label="Dismiss from next up"
        >
          <RiCloseLine size={14} />
        </button>
      </div>
      <div className={styles.info}>
        <p className={`label-sm ${styles.showTitle}`}>{item.show_title}</p>
        <p className={`label-md ${styles.title}`}>
          <span className={styles.epLabel}>{epLabel} · </span>
          {item.title}
        </p>
      </div>
    </Link>
  )
}

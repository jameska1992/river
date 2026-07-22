import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiArrowRightSLine, RiArrowLeftSLine, RiArrowRightLine } from 'react-icons/ri'
import { api } from '../api'
import type { Collection } from '../api'
import { CollectionPreview } from './CollectionPreview'
import styles from './CollectionsCarousel.module.css'

export function CollectionsCarousel() {
  const [collections, setCollections] = useState<Collection[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    api.listCollections()
      .then(setCollections)
      .finally(() => setLoaded(true))
  }, [])

  if (!loaded) return null
  if (collections.length === 0) return null

  return (
    <section className={styles.section}>
      <div className={styles.header}>
        <Link to="/collections" className={styles.titleLink}>
          <span className={`headline-sm ${styles.title}`}>Collections</span>
          <RiArrowRightLine size={18} className={styles.titleArrow} />
        </Link>
      </div>
      <CollectionTrack collections={collections} />
    </section>
  )
}

function CollectionTrack({ collections }: { collections: Collection[] }) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)
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
  }, [collections])

  const scroll = (dir: -1 | 1) => {
    const el = trackRef.current
    if (!el) return
    el.scrollBy({ left: el.clientWidth * 0.8 * dir, behavior: 'smooth' })
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
        {collections.map(col => (
          <div key={col.id} className={styles.item}>
            <CollectionCard collection={col} />
          </div>
        ))}

      </div>
    </div>
  )
}

function CollectionCard({ collection }: { collection: Collection }) {
  return (
    <Link to={`/collections/${collection.id}`} className={styles.card}>
      <CollectionPreview covers={collection.covers ?? []} name={collection.name} />
      <div className={styles.cardMeta}>
        <span className={styles.cardName}>{collection.name}</span>
        <span className={styles.cardCount}>
          {collection.item_count} {collection.item_count === 1 ? 'item' : 'items'}
        </span>
      </div>
    </Link>
  )
}

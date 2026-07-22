import { useCallback, useEffect, useState } from 'react'
import { RiFilmLine, RiTv2Line, RiSearchLine, RiCheckLine, RiRefreshLine } from 'react-icons/ri'
import { api } from '../../api'
import type { UnidentifiedItem } from '../../api'
import { IdentifyMovieModal } from '../../components/IdentifyMovieModal'
import { IdentifyTVShowModal } from '../../components/IdentifyTVShowModal'
import styles from './UnidentifiedPage.module.css'

export function UnidentifiedPage() {
  const [items, setItems] = useState<UnidentifiedItem[]>([])
  const [identifying, setIdentifying] = useState<UnidentifiedItem | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const load = useCallback((indicator: 'initial' | 'refresh') => {
    if (indicator === 'initial') setLoading(true)
    else setRefreshing(true)
    api.getUnidentifiedMedia()
      .then(setItems)
      .catch(() => setItems([]))
      .finally(() => {
        setLoading(false)
        setRefreshing(false)
      })
  }, [])

  useEffect(() => { load('initial') }, [load])

  // After identifying succeeds, give the meta worker a moment to enrich
  // before re-fetching so the row drops off the list naturally.
  const handleSubmitted = () => setTimeout(() => load('refresh'), 1500)

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-lg ${styles.heading}`}>Unidentified Media</h1>
        <div className={styles.headerActions}>
          <span className={`label-sm ${styles.count}`}>
            {loading
              ? 'Loading…'
              : items.length === 0
                ? 'All matched'
                : `${items.length} item${items.length === 1 ? '' : 's'}`}
          </span>
          <button
            className="btn btn-secondary"
            onClick={() => load('refresh')}
            disabled={loading || refreshing}
            aria-label="Refresh"
          >
            <RiRefreshLine size={16} className={refreshing ? styles.spinning : undefined} />
            Refresh
          </button>
        </div>
      </div>

      {!loading && items.length === 0 ? (
        <div className={styles.empty}>
          <RiCheckLine size={20} />
          <span className="body-sm">Nothing waiting — every media record has metadata.</span>
        </div>
      ) : (
        <div className={styles.list}>
          {items.map(item => (
            <div key={`${item.type}-${item.id}`} className={`surface ${styles.listItem}`}>
              <span
                className={styles.listIcon}
                style={{ color: item.type === 'movie' ? 'var(--color-primary)' : 'var(--color-secondary)' }}
              >
                {item.type === 'movie' ? <RiFilmLine /> : <RiTv2Line />}
              </span>
              <div className={styles.listMeta}>
                <span className={`label-md ${styles.itemTitle}`}>
                  {item.title || <em className={styles.dim}>(untitled)</em>}
                  {item.year > 0 && <span className={styles.dim}> ({item.year})</span>}
                </span>
                <span className={`label-sm ${styles.itemSub}`}>
                  {item.type === 'movie' ? 'Movie' : 'TV Show'}
                  {item.file_path && <> · {item.file_path}</>}
                </span>
              </div>
              <button
                className="btn btn-secondary"
                onClick={() => setIdentifying(item)}
                title="Identify this item"
              >
                <RiSearchLine size={16} />
                Identify
              </button>
            </div>
          ))}
        </div>
      )}

      {identifying?.type === 'movie' && (
        <IdentifyMovieModal
          movie={{ id: identifying.id, title: identifying.title, year: identifying.year }}
          onClose={() => setIdentifying(null)}
          onSubmitted={handleSubmitted}
        />
      )}
      {identifying?.type === 'tvshow' && (
        <IdentifyTVShowModal
          show={{ id: identifying.id, title: identifying.title, year: identifying.year }}
          onClose={() => setIdentifying(null)}
          onSubmitted={handleSubmitted}
        />
      )}
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { TVShow } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

interface Props {
  show: Pick<TVShow, 'id' | 'title' | 'year'>
  onClose: () => void
  onSubmitted?: () => void
}

const imdbRe = /^tt\d{6,10}$/

// IdentifyTVShowModal: admin override when the on-disk folder name doesn't
// resolve cleanly. Title and year persist on the show record; the IMDb id is
// a one-shot hint that routes through TMDB's /find lookup.
export function IdentifyTVShowModal({ show, onClose, onSubmitted }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [title, setTitle]   = useState(show.title ?? '')
  const [year, setYear]     = useState(show.year ? String(show.year) : '')
  const [imdbID, setImdbID] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError]   = useState<string | null>(null)

  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const trimmedImdb = imdbID.trim().toLowerCase()
    if (trimmedImdb && !imdbRe.test(trimmedImdb)) {
      setError('IMDb id must look like tt1234567')
      return
    }
    const yearNum = year.trim() ? parseInt(year, 10) : NaN
    if (year.trim() && (isNaN(yearNum) || yearNum < 1800 || yearNum > 2200)) {
      setError('Year must be a 4-digit number')
      return
    }
    const titleTrimmed = title.trim()
    if (!titleTrimmed && !trimmedImdb) {
      setError('Provide a title, IMDb id, or both')
      return
    }

    setSubmitting(true)
    try {
      await api.identifyTVShow(show.id, {
        title: titleTrimmed !== show.title ? titleTrimmed : undefined,
        year:  !isNaN(yearNum) && yearNum !== show.year ? yearNum : undefined,
        imdb_id: trimmedImdb || undefined,
      })
      onSubmitted?.()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to identify show')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className={styles.overlay}
      ref={overlayRef}
      onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.header}>
          <h2 className={`headline-sm ${styles.headerTitle}`}>Identify TV show</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>
            <p className="body-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
              Override what the metadata enhancer uses to look up this show. Title and year are
              saved on the record; IMDb id is used once to disambiguate via TMDB's /find lookup.
            </p>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>Title</span>
              <input
                className={styles.input}
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder="Breaking Bad"
              />
            </label>

            <div className={styles.row}>
              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>First-aired year</span>
                <input
                  className={styles.input}
                  type="number"
                  min="1800"
                  max="2200"
                  value={year}
                  onChange={e => setYear(e.target.value)}
                  placeholder="2008"
                />
              </label>

              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>
                  IMDb id <span className={styles.hint}>(e.g. tt0903747)</span>
                </span>
                <input
                  className={styles.input}
                  value={imdbID}
                  onChange={e => setImdbID(e.target.value)}
                  placeholder="tt0903747"
                  autoCapitalize="off"
                  autoCorrect="off"
                  spellCheck={false}
                />
              </label>
            </div>
          </div>

          {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={submitting}>
              {submitting ? 'Identifying…' : 'Identify & re-scan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

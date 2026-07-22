import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { Movie } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

// Only the three fields the form uses — narrower than Movie so the admin
// "unidentified" list can pass minimal items without synthesizing a full Movie.
interface Props {
  movie: Pick<Movie, 'id' | 'title' | 'year'>
  onClose: () => void
  onSubmitted?: () => void
}

const imdbRe = /^tt\d{6,10}$/

// IdentifyMovieModal: admin override for when the on-disk title doesn't
// match TMDB cleanly. Title and year persist on the movie record; the IMDb
// id is a one-shot hint used by river-meta-movie's /find lookup.
export function IdentifyMovieModal({ movie, onClose, onSubmitted }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [title, setTitle]   = useState(movie.title ?? '')
  const [year, setYear]     = useState(movie.year ? String(movie.year) : '')
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
      await api.identifyMovie(movie.id, {
        title: titleTrimmed !== movie.title ? titleTrimmed : undefined,
        year:  !isNaN(yearNum) && yearNum !== movie.year ? yearNum : undefined,
        imdb_id: trimmedImdb || undefined,
      })
      onSubmitted?.()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to identify movie')
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Identify movie</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>
            <p className="body-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
              Override what the metadata enhancer uses to look up this movie. Title and year are
              saved on the record; IMDb id is used once to disambiguate via TMDB's /find lookup.
            </p>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>Title</span>
              <input
                className={styles.input}
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder="The Matrix"
              />
            </label>

            <div className={styles.row}>
              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Release year</span>
                <input
                  className={styles.input}
                  type="number"
                  min="1800"
                  max="2200"
                  value={year}
                  onChange={e => setYear(e.target.value)}
                  placeholder="1999"
                />
              </label>

              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>
                  IMDb id <span className={styles.hint}>(e.g. tt0133093)</span>
                </span>
                <input
                  className={styles.input}
                  value={imdbID}
                  onChange={e => setImdbID(e.target.value)}
                  placeholder="tt0133093"
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

import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { Season } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

interface Props {
  showId: string
  season: Season
  onSave: (updated: Season) => void
  onClose: () => void
}

// SeasonMetadataModal: admin override for a single season's metadata.
// Mostly useful when meta-tv couldn't find the season on TMDB (newly-aired
// season, fan-made compilation, etc.) and the admin wants to set a poster,
// title, or fix the season number for misnamed folders ("Season 0" specials
// being detected as Season 1, etc.).
export function SeasonMetadataModal({ showId, season, onSave, onClose }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [number, setNumber] = useState(String(season.number || ''))
  const [title, setTitle] = useState(season.title ?? '')
  const [description, setDescription] = useState(season.description ?? '')
  const [year, setYear] = useState(String(season.year || ''))
  const [posterPath, setPosterPath] = useState(season.poster_path ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const numberNum = number.trim() ? parseInt(number, 10) : NaN
    if (number.trim() && (isNaN(numberNum) || numberNum < 0 || numberNum > 999)) {
      setError('Season number must be a non-negative integer')
      return
    }
    const yearNum = year.trim() ? parseInt(year, 10) : NaN
    if (year.trim() && (isNaN(yearNum) || yearNum < 1800 || yearNum > 2200)) {
      setError('Year must be a 4-digit number')
      return
    }

    setSubmitting(true)
    try {
      const updated = await api.updateSeason(showId, season.id, {
        number: !isNaN(numberNum) && numberNum !== season.number ? numberNum : undefined,
        title: title !== (season.title ?? '') ? title : undefined,
        description: description !== (season.description ?? '') ? description : undefined,
        year: !isNaN(yearNum) && yearNum !== season.year ? yearNum : undefined,
        poster_path: posterPath !== (season.poster_path ?? '') ? posterPath : undefined,
      })
      onSave(updated)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update season')
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Edit season</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>
            <div className={styles.row}>
              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Season number</span>
                <input
                  className={styles.input}
                  type="number"
                  min="0"
                  max="999"
                  value={number}
                  onChange={e => setNumber(e.target.value)}
                />
              </label>

              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Year</span>
                <input
                  className={styles.input}
                  type="number"
                  min="1800"
                  max="2200"
                  value={year}
                  onChange={e => setYear(e.target.value)}
                />
              </label>
            </div>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>Title</span>
              <input
                className={styles.input}
                value={title}
                onChange={e => setTitle(e.target.value)}
              />
            </label>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>Description</span>
              <textarea
                className={styles.textarea}
                value={description}
                onChange={e => setDescription(e.target.value)}
                rows={4}
              />
            </label>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>Poster URL</span>
              <input
                className={styles.input}
                type="url"
                value={posterPath}
                onChange={e => setPosterPath(e.target.value)}
              />
            </label>
          </div>

          {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

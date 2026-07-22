import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { Episode, Season } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

interface Props {
  showId: string
  // The episode being edited and the season it currently belongs to. The
  // season is needed because the API route is keyed by it; once the admin
  // re-parents the episode the PUT still goes to the *current* season.
  seasonId: string
  episode: Episode
  // All seasons under this show, so the admin can re-parent the episode
  // when the scanner classified it wrong. Optional — if omitted, the season
  // selector is hidden and only same-season edits are possible.
  seasons?: Season[]
  onSave: (updated: Episode) => void
  onClose: () => void
}

// EpisodeMetadataModal: admin override for a single episode's metadata.
// Beyond the usual title/description/runtime/aired_at, this also exposes
// the episode number and a season picker — both are common scanner-mis-
// detection symptoms (filename SxxExx parse failures or off-by-one
// numbering, multi-episode files merged into a single episode, etc.).
export function EpisodeMetadataModal({ showId, seasonId, episode, seasons, onSave, onClose }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [number, setNumber] = useState(String(episode.number || ''))
  const [seasonSel, setSeasonSel] = useState(seasonId)
  const [title, setTitle] = useState(episode.title ?? '')
  const [description, setDescription] = useState(episode.description ?? '')
  const [runtime, setRuntime] = useState(String(episode.runtime || ''))
  // The model stores aired_at as RFC3339; the date input wants YYYY-MM-DD.
  const [airedAt, setAiredAt] = useState(
    episode.aired_at ? episode.aired_at.slice(0, 10) : ''
  )
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
    if (number.trim() && (isNaN(numberNum) || numberNum < 0 || numberNum > 9999)) {
      setError('Episode number must be a non-negative integer')
      return
    }
    const runtimeNum = runtime.trim() ? parseInt(runtime, 10) : NaN
    if (runtime.trim() && (isNaN(runtimeNum) || runtimeNum < 0)) {
      setError('Runtime must be a non-negative integer')
      return
    }
    // Build an RFC3339 timestamp from the YYYY-MM-DD input. The empty
    // string means "no change" — explicitly clearing the field would need
    // a different signal which we don't support here.
    let airedAtRFC = ''
    if (airedAt && airedAt !== episode.aired_at?.slice(0, 10)) {
      airedAtRFC = `${airedAt}T00:00:00Z`
    }

    setSubmitting(true)
    try {
      // Send only fields that actually changed — the server applies PATCH
      // semantics on each field anyway, but keeping the payload minimal
      // makes the audit trail cleaner.
      const updated = await api.updateEpisode(showId, seasonId, episode.id, {
        number: !isNaN(numberNum) && numberNum !== episode.number ? numberNum : undefined,
        season_id: seasonSel !== seasonId ? seasonSel : undefined,
        title: title !== (episode.title ?? '') ? title : undefined,
        description: description !== (episode.description ?? '') ? description : undefined,
        runtime: !isNaN(runtimeNum) && runtimeNum !== episode.runtime ? runtimeNum : undefined,
        aired_at: airedAtRFC || undefined,
      })
      onSave(updated)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update episode')
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Edit episode</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>
            <div className={styles.row}>
              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Episode number</span>
                <input
                  className={styles.input}
                  type="number"
                  min="0"
                  max="9999"
                  value={number}
                  onChange={e => setNumber(e.target.value)}
                />
              </label>

              {seasons && seasons.length > 1 && (
                <label className={styles.field}>
                  <span className={`label-sm ${styles.label}`}>Season</span>
                  <select
                    className={styles.input}
                    value={seasonSel}
                    onChange={e => setSeasonSel(e.target.value)}
                  >
                    {seasons.map(s => (
                      <option key={s.id} value={s.id}>
                        Season {s.number}{s.title ? ` — ${s.title}` : ''}
                      </option>
                    ))}
                  </select>
                </label>
              )}
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

            <div className={styles.row}>
              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Runtime (minutes)</span>
                <input
                  className={styles.input}
                  type="number"
                  min="0"
                  value={runtime}
                  onChange={e => setRuntime(e.target.value)}
                />
              </label>

              <label className={styles.field}>
                <span className={`label-sm ${styles.label}`}>Aired</span>
                <input
                  className={styles.input}
                  type="date"
                  value={airedAt}
                  onChange={e => setAiredAt(e.target.value)}
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
              {submitting ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

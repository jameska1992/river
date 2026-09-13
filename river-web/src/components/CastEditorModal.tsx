import { useEffect, useRef, useState } from 'react'
import { RiCloseLine, RiAddLine, RiDeleteBinLine, RiArrowUpLine, RiArrowDownLine, RiUserLine } from 'react-icons/ri'
import type { Credits, SetCreditsRequest } from '../api'
import { api, ApiError } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './CastEditorModal.module.css'

interface Props {
  type: 'movie' | 'tvshow'
  mediaId: string
  // Full current credits. Crew is preserved verbatim on save — the PUT
  // endpoint replaces cast AND crew, so submitting cast alone would wipe it.
  credits: Credits
  onSaved: (updated: Credits) => void
  onClose: () => void
}

// One editable cast row. `key` is a stable client-side id so React keeps
// input focus correct across reorders/removals. `tmdbId` is carried through
// for TMDB-sourced people so a re-save dedupes onto the existing Person
// instead of creating a duplicate; manually-added rows leave it null and the
// backend mints a fresh Person (tmdb_id stays NULL, no unique-index clash).
interface CastRow {
  key: number
  tmdbId: number | null
  name: string
  character: string
  profilePath: string
}

export function CastEditorModal({ type, mediaId, credits, onSaved, onClose }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  // Monotonic id source for row keys. Seeded past the initial rows (which
  // key off their index) so freshly-added rows never collide with them.
  const nextKey = useRef(credits.cast.length)

  const [rows, setRows] = useState<CastRow[]>(() =>
    credits.cast.map((c, i) => ({
      key: i,
      tmdbId: c.tmdb_id,
      name: c.name,
      character: c.character,
      profilePath: c.profile_path,
    })),
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const update = (key: number, patch: Partial<CastRow>) =>
    setRows(rs => rs.map(r => (r.key === key ? { ...r, ...patch } : r)))

  const remove = (key: number) => setRows(rs => rs.filter(r => r.key !== key))

  const move = (index: number, delta: number) =>
    setRows(rs => {
      const next = index + delta
      if (next < 0 || next >= rs.length) return rs
      const copy = [...rs]
      ;[copy[index], copy[next]] = [copy[next], copy[index]]
      return copy
    })

  const add = () =>
    setRows(rs => [...rs, { key: nextKey.current++, tmdbId: null, name: '', character: '', profilePath: '' }])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // Drop rows the user left completely blank, then require a name on
    // whatever remains (the API binds name as required).
    const kept = rows.filter(r => r.name.trim() || r.character.trim() || r.profilePath.trim())
    if (kept.some(r => !r.name.trim())) {
      setError('Every cast member needs a name. Remove empty rows or fill them in.')
      return
    }

    const payload: SetCreditsRequest = {
      cast: kept.map((r, i) => ({
        ...(r.tmdbId ? { tmdb_id: r.tmdbId } : {}),
        name: r.name.trim(),
        profile_path: r.profilePath.trim(),
        character: r.character.trim(),
        order: i,
      })),
      // Preserve crew untouched — the endpoint is full-replace.
      crew: credits.crew.map(c => ({
        ...(c.tmdb_id ? { tmdb_id: c.tmdb_id } : {}),
        name: c.name,
        profile_path: c.profile_path,
        job: c.job,
        department: c.department,
      })),
    }

    setSaving(true)
    setError(null)
    try {
      if (type === 'movie') await api.setMovieCredits(mediaId, payload)
      else await api.setTVShowCredits(mediaId, payload)
      // 204 carries no body — re-fetch so we render resolved person ids.
      const updated = type === 'movie'
        ? await api.getMovieCredits(mediaId)
        : await api.getTVShowCredits(mediaId)
      onSaved(updated)
      onClose()
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError('You don’t have permission to edit cast (admin only).')
      } else {
        setError(err instanceof Error ? err.message : 'Failed to save cast')
      }
    } finally {
      setSaving(false)
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Edit cast</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.rows}>
            {rows.length === 0 && (
              <p className={`body-md ${styles.empty}`}>No cast members yet. Add one below.</p>
            )}
            {rows.map((r, i) => (
              <div key={r.key} className={styles.row}>
                <div className={styles.photo}>
                  {imageUrl(r.profilePath) ? (
                    <img src={imageUrl(r.profilePath)} alt="" />
                  ) : (
                    <RiUserLine size={20} />
                  )}
                </div>
                <div className={styles.fields}>
                  <input
                    className={styles.input}
                    value={r.name}
                    onChange={e => update(r.key, { name: e.target.value })}
                    placeholder="Name"
                    aria-label="Name"
                  />
                  <input
                    className={styles.input}
                    value={r.character}
                    onChange={e => update(r.key, { character: e.target.value })}
                    placeholder="Character"
                    aria-label="Character"
                  />
                  <input
                    className={styles.input}
                    type="url"
                    value={r.profilePath}
                    onChange={e => update(r.key, { profilePath: e.target.value })}
                    placeholder="Profile image URL (optional)"
                    aria-label="Profile image URL"
                  />
                </div>
                <div className={styles.rowActions}>
                  <button
                    type="button"
                    className="btn btn-icon"
                    onClick={() => move(i, -1)}
                    disabled={i === 0}
                    aria-label="Move up"
                    title="Move up"
                  >
                    <RiArrowUpLine size={16} />
                  </button>
                  <button
                    type="button"
                    className="btn btn-icon"
                    onClick={() => move(i, 1)}
                    disabled={i === rows.length - 1}
                    aria-label="Move down"
                    title="Move down"
                  >
                    <RiArrowDownLine size={16} />
                  </button>
                  <button
                    type="button"
                    className={`btn btn-icon ${styles.removeBtn}`}
                    onClick={() => remove(r.key)}
                    aria-label="Remove cast member"
                    title="Remove"
                  >
                    <RiDeleteBinLine size={16} />
                  </button>
                </div>
              </div>
            ))}

            <button type="button" className={`btn ${styles.addBtn}`} onClick={add}>
              <RiAddLine size={16} />
              <span>Add cast member</span>
            </button>
          </div>

          {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

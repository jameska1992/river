import { useEffect, useRef, useState } from 'react'
import { RiCloseLine, RiAddLine, RiDeleteBinLine, RiArrowUpLine, RiArrowDownLine, RiUserLine } from 'react-icons/ri'
import type { Credits, SetCreditsRequest } from '../api'
import { api, ApiError } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './CastEditorModal.module.css'

interface Props {
  type: 'movie' | 'tvshow'
  mediaId: string
  // Full current credits (cast + crew). Both lists are editable here and the
  // full set is re-submitted on save, since the PUT endpoint is full-replace.
  credits: Credits
  onSaved: (updated: Credits) => void
  onClose: () => void
}

// `key` is a stable client-side id so React keeps input focus across
// reorders/removals. `tmdbId` is carried through for TMDB-sourced people so a
// re-save dedupes onto the existing Person; manual rows leave it null and the
// backend mints a fresh Person (orphans from prior edits are reaped server-side).
interface CastRow {
  key: number
  tmdbId: number | null
  name: string
  character: string
  profilePath: string
}

interface CrewRow {
  key: number
  tmdbId: number | null
  name: string
  job: string
  department: string
  profilePath: string
}

type Tab = 'cast' | 'crew'

export function CastEditorModal({ type, mediaId, credits, onSaved, onClose }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  // Seed past both initial lists (cast keys 0..n-1, crew keys n..n+m-1) so
  // freshly-added rows never collide.
  const nextKey = useRef(credits.cast.length + credits.crew.length)

  const [castRows, setCastRows] = useState<CastRow[]>(() =>
    credits.cast.map((c, i) => ({
      key: i,
      tmdbId: c.tmdb_id,
      name: c.name,
      character: c.character,
      profilePath: c.profile_path,
    })),
  )
  const [crewRows, setCrewRows] = useState<CrewRow[]>(() =>
    credits.crew.map((c, i) => ({
      key: credits.cast.length + i,
      tmdbId: c.tmdb_id,
      name: c.name,
      job: c.job,
      department: c.department,
      profilePath: c.profile_path,
    })),
  )
  const [tab, setTab] = useState<Tab>('cast')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  // ── Cast ops ──
  const updateCast = (key: number, patch: Partial<CastRow>) =>
    setCastRows(rs => rs.map(r => (r.key === key ? { ...r, ...patch } : r)))
  const removeCast = (key: number) => setCastRows(rs => rs.filter(r => r.key !== key))
  const moveCast = (index: number, delta: number) =>
    setCastRows(rs => {
      const next = index + delta
      if (next < 0 || next >= rs.length) return rs
      const copy = [...rs]
      ;[copy[index], copy[next]] = [copy[next], copy[index]]
      return copy
    })
  const addCast = () =>
    setCastRows(rs => [...rs, { key: nextKey.current++, tmdbId: null, name: '', character: '', profilePath: '' }])

  // ── Crew ops ── (crew has no display order — server sorts by dept/job)
  const updateCrew = (key: number, patch: Partial<CrewRow>) =>
    setCrewRows(rs => rs.map(r => (r.key === key ? { ...r, ...patch } : r)))
  const removeCrew = (key: number) => setCrewRows(rs => rs.filter(r => r.key !== key))
  const addCrew = () =>
    setCrewRows(rs => [...rs, { key: nextKey.current++, tmdbId: null, name: '', job: '', department: '', profilePath: '' }])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // Drop fully-blank rows, then require a name on whatever remains (the API
    // binds name as required for both cast and crew).
    const keptCast = castRows.filter(r => r.name.trim() || r.character.trim() || r.profilePath.trim())
    const keptCrew = crewRows.filter(r => r.name.trim() || r.job.trim() || r.department.trim() || r.profilePath.trim())
    if (keptCast.some(r => !r.name.trim())) {
      setTab('cast')
      setError('Every cast member needs a name. Remove empty rows or fill them in.')
      return
    }
    if (keptCrew.some(r => !r.name.trim())) {
      setTab('crew')
      setError('Every crew member needs a name. Remove empty rows or fill them in.')
      return
    }

    const payload: SetCreditsRequest = {
      cast: keptCast.map((r, i) => ({
        ...(r.tmdbId ? { tmdb_id: r.tmdbId } : {}),
        name: r.name.trim(),
        profile_path: r.profilePath.trim(),
        character: r.character.trim(),
        order: i,
      })),
      crew: keptCrew.map(r => ({
        ...(r.tmdbId ? { tmdb_id: r.tmdbId } : {}),
        name: r.name.trim(),
        profile_path: r.profilePath.trim(),
        job: r.job.trim(),
        department: r.department.trim(),
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
        setError('You don’t have permission to edit credits (admin only).')
      } else {
        setError(err instanceof Error ? err.message : 'Failed to save credits')
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Edit credits</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <div className={styles.tabs} role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'cast'}
            className={`${styles.tab} ${tab === 'cast' ? styles.tabActive : ''}`}
            onClick={() => setTab('cast')}
          >
            Cast {castRows.length > 0 && <span className={styles.tabCount}>{castRows.length}</span>}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'crew'}
            className={`${styles.tab} ${tab === 'crew' ? styles.tabActive : ''}`}
            onClick={() => setTab('crew')}
          >
            Crew {crewRows.length > 0 && <span className={styles.tabCount}>{crewRows.length}</span>}
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.rows}>
            {tab === 'cast' ? (
              <>
                {castRows.length === 0 && (
                  <p className={`body-md ${styles.empty}`}>No cast members yet. Add one below.</p>
                )}
                {castRows.map((r, i) => (
                  <div key={r.key} className={styles.row}>
                    <div className={styles.photo}>
                      {imageUrl(r.profilePath) ? <img src={imageUrl(r.profilePath)} alt="" /> : <RiUserLine size={20} />}
                    </div>
                    <div className={styles.fields}>
                      <input className={styles.input} value={r.name} onChange={e => updateCast(r.key, { name: e.target.value })} placeholder="Name" aria-label="Name" />
                      <input className={styles.input} value={r.character} onChange={e => updateCast(r.key, { character: e.target.value })} placeholder="Character" aria-label="Character" />
                      <input className={styles.input} type="url" value={r.profilePath} onChange={e => updateCast(r.key, { profilePath: e.target.value })} placeholder="Profile image URL (optional)" aria-label="Profile image URL" />
                    </div>
                    <div className={styles.rowActions}>
                      <button type="button" className="btn btn-icon" onClick={() => moveCast(i, -1)} disabled={i === 0} aria-label="Move up" title="Move up"><RiArrowUpLine size={16} /></button>
                      <button type="button" className="btn btn-icon" onClick={() => moveCast(i, 1)} disabled={i === castRows.length - 1} aria-label="Move down" title="Move down"><RiArrowDownLine size={16} /></button>
                      <button type="button" className={`btn btn-icon ${styles.removeBtn}`} onClick={() => removeCast(r.key)} aria-label="Remove cast member" title="Remove"><RiDeleteBinLine size={16} /></button>
                    </div>
                  </div>
                ))}
                <button type="button" className={`btn ${styles.addBtn}`} onClick={addCast}>
                  <RiAddLine size={16} /><span>Add cast member</span>
                </button>
              </>
            ) : (
              <>
                {crewRows.length === 0 && (
                  <p className={`body-md ${styles.empty}`}>No crew members yet. Add one below.</p>
                )}
                {crewRows.map(r => (
                  <div key={r.key} className={styles.row}>
                    <div className={styles.photo}>
                      {imageUrl(r.profilePath) ? <img src={imageUrl(r.profilePath)} alt="" /> : <RiUserLine size={20} />}
                    </div>
                    <div className={styles.fields}>
                      <input className={styles.input} value={r.name} onChange={e => updateCrew(r.key, { name: e.target.value })} placeholder="Name" aria-label="Name" />
                      <input className={styles.input} value={r.job} onChange={e => updateCrew(r.key, { job: e.target.value })} placeholder="Job (e.g. Director)" aria-label="Job" />
                      <input className={styles.input} value={r.department} onChange={e => updateCrew(r.key, { department: e.target.value })} placeholder="Department (e.g. Directing)" aria-label="Department" />
                      <input className={styles.input} type="url" value={r.profilePath} onChange={e => updateCrew(r.key, { profilePath: e.target.value })} placeholder="Profile image URL (optional)" aria-label="Profile image URL" />
                    </div>
                    <div className={styles.rowActions}>
                      <button type="button" className={`btn btn-icon ${styles.removeBtn}`} onClick={() => removeCrew(r.key)} aria-label="Remove crew member" title="Remove"><RiDeleteBinLine size={16} /></button>
                    </div>
                  </div>
                ))}
                <button type="button" className={`btn ${styles.addBtn}`} onClick={addCrew}>
                  <RiAddLine size={16} /><span>Add crew member</span>
                </button>
              </>
            )}
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

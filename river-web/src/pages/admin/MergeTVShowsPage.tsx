import { useCallback, useEffect, useMemo, useState } from 'react'
import { RiArrowRightLine, RiGitMergeLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { MergePreview, TVShow } from '../../api'
import styles from './MergeTVShowsPage.module.css'

// fetchAllShows pages through every show so the picker can offer all of them,
// not just the first page. Duplicates share a title, so we can't rely on a
// bounded list.
async function fetchAllShows(): Promise<TVShow[]> {
  const all: TVShow[] = []
  const limit = 200
  for (let page = 1; ; page++) {
    const { items, total } = await api.listTVShowsPaged({ page, limit, sort: 'title' })
    all.push(...items)
    if (items.length === 0 || all.length >= total) break
  }
  return all
}

function showLabel(s: TVShow): string {
  const year = s.year ? ` (${s.year})` : ''
  return `${s.title}${year} — ${s.folder_path || 'no path'}`
}

export function MergeTVShowsPage() {
  const [shows, setShows] = useState<TVShow[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  const [idA, setIdA] = useState('')
  const [idB, setIdB] = useState('')

  const [preview, setPreview] = useState<MergePreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [merging, setMerging] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // applyShows never sets state synchronously — the effect relies on the
  // initial loading=true, and the post-merge reload flips it on itself.
  const applyShows = useCallback(() => {
    return fetchAllShows()
      .then(list => { setShows(list); setLoadError(null) })
      .catch(err => setLoadError(err instanceof Error ? err.message : 'Failed to load shows'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { void applyShows() }, [applyShows])

  function reloadShows() {
    setLoading(true)
    void applyShows()
  }

  const sorted = useMemo(
    () => [...shows].sort((a, b) =>
      a.title.localeCompare(b.title) || a.folder_path.localeCompare(b.folder_path)),
    [shows],
  )

  const bothPicked = idA !== '' && idB !== '' && idA !== idB

  function resetResult() {
    setPreview(null)
    setSuccess(null)
    setActionError(null)
  }

  function handlePreview() {
    if (!bothPicked) return
    setPreviewing(true)
    setActionError(null)
    setSuccess(null)
    api.previewMergeTVShows([idA, idB])
      .then(setPreview)
      .catch(err => {
        setPreview(null)
        setActionError(err instanceof ApiError ? err.message : 'Preview failed')
      })
      .finally(() => setPreviewing(false))
  }

  function handleMerge() {
    if (!preview?.can_merge) return
    setMerging(true)
    setActionError(null)
    api.mergeTVShows([idA, idB])
      .then(survivor => {
        setSuccess(`Merged into “${survivor.title}”. New episodes under the absorbed path will attach here on the next scan.`)
        setPreview(null)
        setIdA('')
        setIdB('')
        reloadShows()
      })
      .catch(err => setActionError(err instanceof ApiError ? err.message : 'Merge failed'))
      .finally(() => setMerging(false))
  }

  return (
    <div>
      <div className={styles.header}>
        <h1 className="headline-lg">Merge TV Shows</h1>
      </div>
      <p className={`body-md ${styles.intro}`}>
        Combine two records for the same series — e.g. when its seasons are split across
        directory roots. The <strong>older</strong> record is kept and absorbs the other’s
        seasons and episodes; the absorbed folder is remembered so future scans attach new
        episodes to the survivor. This can’t be undone, so confirm it really is the same show.
      </p>

      {loadError && <p className={styles.pageError}>{loadError}</p>}

      {loading ? (
        <p className="label-sm">Loading shows…</p>
      ) : sorted.length < 2 ? (
        <div className={`surface-low ${styles.empty}`}>You need at least two shows to merge.</div>
      ) : (
        <>
          <div className={styles.pickers}>
            <label className={styles.picker}>
              <span className="label-md">First show</span>
              <select className="input" value={idA} onChange={e => { setIdA(e.target.value); resetResult() }}>
                <option value="">Select a show…</option>
                {sorted.map(s => (
                  <option key={s.id} value={s.id} disabled={s.id === idB}>{showLabel(s)}</option>
                ))}
              </select>
            </label>
            <label className={styles.picker}>
              <span className="label-md">Second show</span>
              <select className="input" value={idB} onChange={e => { setIdB(e.target.value); resetResult() }}>
                <option value="">Select a show…</option>
                {sorted.map(s => (
                  <option key={s.id} value={s.id} disabled={s.id === idA}>{showLabel(s)}</option>
                ))}
              </select>
            </label>
          </div>

          <div className={styles.actions}>
            <button className="btn btn-primary" onClick={handlePreview} disabled={!bothPicked || previewing}>
              {previewing ? 'Previewing…' : 'Preview merge'}
            </button>
          </div>

          {actionError && <p className={styles.pageError}>{actionError}</p>}
          {success && <p className={styles.success}>{success}</p>}

          {preview && (
            <div className={`surface-low ${styles.preview}`}>
              <div className={styles.flow}>
                <div className={styles.flowCard}>
                  <span className="label-sm">Absorbed (removed)</span>
                  <strong>{preview.merged.title}</strong>
                  <span className={styles.path}>{preview.merged.folder_path || 'no path'}</span>
                </div>
                <RiArrowRightLine className={styles.flowArrow} aria-hidden />
                <div className={`${styles.flowCard} ${styles.survivorCard}`}>
                  <span className="label-sm">Survivor (kept)</span>
                  <strong>{preview.survivor.title}</strong>
                  <span className={styles.path}>{preview.survivor.folder_path || 'no path'}</span>
                </div>
              </div>

              <p className="body-md">
                Moving <strong>{preview.seasons_moved}</strong> season(s) and{' '}
                <strong>{preview.episodes_moved}</strong> episode(s) to the survivor.
              </p>

              {preview.conflicts.length > 0 && (
                <div className={styles.conflicts}>
                  <p className="label-md">
                    {preview.conflicts.length} conflicting episode(s) block this merge:
                  </p>
                  <ul>
                    {preview.conflicts.map((c, i) => (
                      <li key={i}>
                        S{c.season_number} · {c.is_special ? 'Special ' : 'E'}{c.episode_number}
                        {c.title ? ` — ${c.title}` : ''}
                      </li>
                    ))}
                  </ul>
                  <p className="label-sm">Both shows have these episodes. Resolve the overlap on disk, then try again.</p>
                </div>
              )}

              <div className={styles.actions}>
                <button className="btn btn-primary" onClick={handleMerge} disabled={!preview.can_merge || merging}>
                  <RiGitMergeLine aria-hidden />
                  {merging ? 'Merging…' : 'Confirm merge'}
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

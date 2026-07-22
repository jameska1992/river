import { useCallback, useEffect, useMemo, useState } from 'react'
import { RiRefreshLine, RiDeleteBin6Line } from 'react-icons/ri'
import { api } from '../../api'
import styles from './ScannerStatePage.module.css'

interface DirectoryRecord {
  library_id:    string
  content_hash:  string
  last_scanned:  string
}

type Tab = 'directories' | 'shows'

interface ConfirmState {
  kind:    'path' | 'prefix' | 'show'
  label:   string
  payload: string
}

export function ScannerStatePage() {
  const [directories, setDirectories] = useState<Record<string, DirectoryRecord>>({})
  const [shows,       setShows]       = useState<Record<string, string>>({})
  const [tab,         setTab]         = useState<Tab>('directories')
  const [filter,      setFilter]      = useState('')
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)
  const [busyKey,     setBusyKey]     = useState<string | null>(null)
  const [confirm,     setConfirm]     = useState<ConfirmState | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const state = await api.getScannerState()
      setDirectories(state.directories ?? {})
      setShows(state.shows ?? {})
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load scanner state')
    } finally {
      setLoading(false)
    }
  }, [])

  // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount; load() sets state from the API response
  useEffect(() => { void load() }, [load])

  const directoryRows = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    return Object.entries(directories)
      .filter(([path]) => !needle || path.toLowerCase().includes(needle))
      .sort(([a], [b]) => a.localeCompare(b))
  }, [directories, filter])

  const showRows = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    return Object.entries(shows)
      .filter(([path, id]) =>
        !needle || path.toLowerCase().includes(needle) || id.toLowerCase().includes(needle))
      .sort(([a], [b]) => a.localeCompare(b))
  }, [shows, filter])

  const doForget = useCallback(async (kind: ConfirmState['kind'], payload: string) => {
    setBusyKey(payload)
    setError(null)
    try {
      if (kind === 'path')   await api.forgetScannerState({ paths:    [payload] })
      if (kind === 'prefix') await api.forgetScannerState({ prefixes: [payload] })
      if (kind === 'show')   await api.forgetScannerState({ shows:    [payload] })
      // Reflect the change locally rather than re-fetching the whole map.
      if (kind === 'path') {
        setDirectories(prev => {
          const next = { ...prev }
          delete next[payload]
          return next
        })
      } else if (kind === 'prefix') {
        setDirectories(prev => {
          const next: typeof prev = {}
          const sep = payload.endsWith('/') ? '' : '/'
          for (const [k, v] of Object.entries(prev)) {
            if (k === payload || k.startsWith(payload + sep)) continue
            next[k] = v
          }
          return next
        })
      } else {
        setShows(prev => {
          const next = { ...prev }
          delete next[payload]
          return next
        })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to forget entry')
    } finally {
      setBusyKey(null)
      setConfirm(null)
    }
  }, [])

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-sm ${styles.heading}`}>Scanner State</h1>
        <div className={styles.actions}>
          <input
            className={styles.filterInput}
            type="search"
            placeholder="Filter…"
            value={filter}
            onChange={e => setFilter(e.target.value)}
          />
          <button
            className={`btn btn-icon ${loading ? 'spinning' : ''}`}
            onClick={() => void load()}
            aria-label="Refresh"
            disabled={loading}
          >
            <RiRefreshLine size={18} />
          </button>
        </div>
      </div>

      <p className={`body-sm ${styles.intro}`}>
        Each directory/file the scanner has visited is stored here with a
        content hash; the scanner uses the hash to skip unchanged content
        on the next run. Remove an entry to force it to be re-scanned —
        useful when a file was modified out-of-band or a downstream
        consumer failed to process its event. The shows map caches a
        show folder path → river-api show ID so re-identification doesn't
        rely on the (mutable) title; remove an entry there if a show was
        deleted and you want the scanner to recreate it.
      </p>

      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${tab === 'directories' ? styles.tabActive : ''}`}
          onClick={() => setTab('directories')}
        >
          Directories ({Object.keys(directories).length})
        </button>
        <button
          className={`${styles.tab} ${tab === 'shows' ? styles.tabActive : ''}`}
          onClick={() => setTab('shows')}
        >
          Shows ({Object.keys(shows).length})
        </button>
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {tab === 'directories' ? (
        <table className={styles.table}>
          <thead>
            <tr>
              <th className={styles.colPath}>Path</th>
              <th className={styles.colHash}>Content hash</th>
              <th className={styles.colTime}>Last scanned</th>
              <th className={styles.colActions} />
            </tr>
          </thead>
          <tbody>
            {directoryRows.length === 0 && !loading && (
              <tr><td colSpan={4} className={styles.empty}>No matching entries.</td></tr>
            )}
            {directoryRows.map(([path, rec]) => (
              <tr key={path}>
                <td className={`${styles.colPath} body-sm`}>{path}</td>
                <td className={`${styles.colHash} label-sm`}>{rec.content_hash.slice(0, 12)}…</td>
                <td className={`${styles.colTime} label-sm`}>
                  {rec.last_scanned ? new Date(rec.last_scanned).toLocaleString(undefined, {
                    month: 'short', day: 'numeric',
                    hour: '2-digit', minute: '2-digit',
                  }) : '—'}
                </td>
                <td className={styles.colActions}>
                  <button
                    className={styles.rowAction}
                    onClick={() => setConfirm({ kind: 'prefix', label: `${path} (prefix)`, payload: path })}
                    disabled={busyKey === path}
                    title="Forget this path and everything beneath it"
                  >
                    Forget tree
                  </button>
                  <button
                    className={`${styles.rowAction} ${styles.rowActionDanger}`}
                    onClick={() => setConfirm({ kind: 'path', label: path, payload: path })}
                    disabled={busyKey === path}
                    aria-label="Forget entry"
                  >
                    <RiDeleteBin6Line size={14} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th className={styles.colPath}>Show folder</th>
              <th className={styles.colHash}>Show ID</th>
              <th className={styles.colActions} />
            </tr>
          </thead>
          <tbody>
            {showRows.length === 0 && !loading && (
              <tr><td colSpan={3} className={styles.empty}>No matching entries.</td></tr>
            )}
            {showRows.map(([path, showId]) => (
              <tr key={path}>
                <td className={`${styles.colPath} body-sm`}>{path}</td>
                <td className={`${styles.colHash} label-sm`}>{showId}</td>
                <td className={styles.colActions}>
                  <button
                    className={`${styles.rowAction} ${styles.rowActionDanger}`}
                    onClick={() => setConfirm({ kind: 'show', label: path, payload: path })}
                    disabled={busyKey === path}
                    aria-label="Forget show mapping"
                  >
                    <RiDeleteBin6Line size={14} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {confirm && (
        <div className={styles.modalBackdrop} onClick={() => setConfirm(null)}>
          <div className={styles.modal} onClick={e => e.stopPropagation()}>
            <h2 className={`title-md ${styles.modalTitle}`}>
              {confirm.kind === 'prefix' ? 'Forget tree?' : 'Forget entry?'}
            </h2>
            <p className={`body-sm ${styles.modalBody}`}>
              {confirm.kind === 'prefix'
                ? 'This removes the content-hash entry for this path AND every entry beneath it. The next scan will re-walk the whole subtree.'
                : confirm.kind === 'path'
                ? 'This removes the content-hash entry for this path. The next scan will treat it as new.'
                : 'This removes the cached show ID. The next scan will re-resolve the show by folder name.'}
            </p>
            <p className={`label-sm ${styles.modalPath}`}>{confirm.label}</p>
            <div className={styles.modalActions}>
              <button className="btn" onClick={() => setConfirm(null)}>Cancel</button>
              <button
                className="btn btn-primary"
                onClick={() => void doForget(confirm.kind, confirm.payload)}
                disabled={busyKey !== null}
              >
                Forget
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

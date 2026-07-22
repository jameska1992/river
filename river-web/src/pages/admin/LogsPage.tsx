import { useEffect, useState, useCallback } from 'react'
import { RiRefreshLine } from 'react-icons/ri'
import { api } from '../../api'
import type { ServiceLog } from '../../api'
import styles from './LogsPage.module.css'

const PAGE_SIZE = 50

const SERVICES = [
  'river-api',
  'river-scan',
  'river-video-trans',
  'river-audio-trans',
  'river-meta-movie',
  'river-meta-tv',
  'river-meta-book',
  'river-meta-music',
]

export function LogsPage() {
  const [logs, setLogs] = useState<ServiceLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [level, setLevel] = useState('')
  const [service, setService] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (p: number) => {
    setLoading(true)
    try {
      const res = await api.getLogs({
        level:   level   || undefined,
        service: service || undefined,
        from:    from    ? new Date(from).toISOString() : undefined,
        to:      to      ? new Date(to).toISOString()   : undefined,
        page: p,
        limit: PAGE_SIZE,
      })
      setLogs(res.logs ?? [])
      setTotal(res.total)
    } catch {
      setLogs([])
    } finally {
      setLoading(false)
    }
  }, [level, service, from, to])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets to the first page and refetches when filters change
    setPage(1)
    void load(1)
  }, [level, service, from, to, load])

  const goToPage = (p: number) => {
    setPage(p)
    void load(p)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-sm ${styles.heading}`}>Service Logs</h1>
        <div className={styles.filters}>
          <select
            className={styles.filterSelect}
            value={level}
            onChange={e => setLevel(e.target.value)}
          >
            <option value="">All levels</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </select>

          <select
            className={styles.filterSelect}
            value={service}
            onChange={e => setService(e.target.value)}
          >
            <option value="">All services</option>
            {SERVICES.map(s => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>

          <input
            type="datetime-local"
            className={styles.filterInput}
            value={from}
            onChange={e => setFrom(e.target.value)}
            title="From"
          />
          <input
            type="datetime-local"
            className={styles.filterInput}
            value={to}
            onChange={e => setTo(e.target.value)}
            title="To"
          />

          <button
            className={`btn btn-icon ${loading ? 'spinning' : ''}`}
            onClick={() => void load(page)}
            aria-label="Refresh"
            disabled={loading}
          >
            <RiRefreshLine size={18} />
          </button>
        </div>
      </div>

      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.colTime}>Time</th>
            <th className={styles.colSvc}>Service</th>
            <th className={styles.colLevel}>Level</th>
            <th className={styles.colMsg}>Message</th>
          </tr>
        </thead>
        <tbody>
          {logs.length === 0 && !loading && (
            <tr>
              <td colSpan={4} className={styles.empty}>No log entries found.</td>
            </tr>
          )}
          {logs.map(entry => (
            <tr key={entry.id}>
              <td className={`${styles.colTime} label-sm`}>
                {new Date(entry.created_at).toLocaleString(undefined, {
                  month: 'short', day: 'numeric',
                  hour: '2-digit', minute: '2-digit', second: '2-digit',
                })}
              </td>
              <td className={`${styles.colSvc} label-sm`}>{entry.service}</td>
              <td className={styles.colLevel}>
                <span className={`${styles.badge} ${levelBadge(entry.level)}`}>
                  {entry.level}
                </span>
              </td>
              <td className={`${styles.colMsg} body-sm`}>{entry.message}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className={styles.pagination}>
        <span className={`label-sm ${styles.pageInfo}`}>
          {total} {total === 1 ? 'entry' : 'entries'}
        </span>
        <div style={{ display: 'flex', gap: 'var(--space-1)' }}>
          <button
            className="btn"
            onClick={() => goToPage(page - 1)}
            disabled={page <= 1}
          >
            Previous
          </button>
          <span className={`label-sm ${styles.pageInfo}`} style={{ alignSelf: 'center', padding: '0 var(--space-2)' }}>
            {page} / {totalPages}
          </span>
          <button
            className="btn"
            onClick={() => goToPage(page + 1)}
            disabled={page >= totalPages}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  )
}

function levelBadge(level: string): string {
  switch (level) {
    case 'warn':  return styles.badgeWarn
    case 'error': return styles.badgeError
    default:      return styles.badgeInfo
  }
}

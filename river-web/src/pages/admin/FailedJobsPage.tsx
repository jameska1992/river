import { useEffect, useState, useCallback } from 'react'
import { RiRefreshLine, RiRestartLine, RiCloseLine } from 'react-icons/ri'
import { api } from '../../api'
import type { FailedJob } from '../../api'
import styles from './FailedJobsPage.module.css'

const PAGE_SIZE = 50

const SERVICES = [
  'river-video-trans',
  'river-audio-trans',
  'river-meta-movie',
  'river-meta-tv',
  'river-meta-book',
  'river-meta-music',
]

export function FailedJobsPage() {
  const [jobs, setJobs] = useState<FailedJob[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [service, setService] = useState('')
  const [loading, setLoading] = useState(false)
  const [acting, setActing] = useState<string | null>(null)

  const load = useCallback(async (p: number) => {
    setLoading(true)
    try {
      const res = await api.getFailedJobs({
        service: service || undefined,
        page: p,
        limit: PAGE_SIZE,
      })
      setJobs(res.jobs ?? [])
      setTotal(res.total)
    } catch {
      setJobs([])
    } finally {
      setLoading(false)
    }
  }, [service])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset to page 1 and refetch when the filter changes
    setPage(1)
    void load(1)
  }, [service, load])

  const goToPage = (p: number) => {
    setPage(p)
    void load(p)
  }

  const retry = async (id: string) => {
    setActing(id)
    try {
      await api.retryFailedJob(id)
      setJobs(prev => prev.filter(j => j.id !== id))
      setTotal(t => Math.max(0, t - 1))
    } catch {
      // leave the row in place on failure; a refresh will re-sync
    } finally {
      setActing(null)
    }
  }

  const dismiss = async (id: string) => {
    setActing(id)
    try {
      await api.dismissFailedJob(id)
      setJobs(prev => prev.filter(j => j.id !== id))
      setTotal(t => Math.max(0, t - 1))
    } catch {
      // no-op
    } finally {
      setActing(null)
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-sm ${styles.heading}`}>Failed Jobs</h1>
        <div className={styles.filters}>
          <select
            className={styles.filterSelect}
            value={service}
            onChange={e => setService(e.target.value)}
          >
            <option value="">All services</option>
            {SERVICES.map(s => <option key={s} value={s}>{s}</option>)}
          </select>
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

      <p className={`body-sm ${styles.blurb}`}>
        Ingest events that were retried and ultimately dead-lettered. Retry re-queues the original
        event; dismiss removes it without retrying.
      </p>

      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.colTime}>Time</th>
            <th className={styles.colSvc}>Service</th>
            <th className={styles.colType}>Type</th>
            <th className={styles.colPath}>Source</th>
            <th className={styles.colReason}>Reason</th>
            <th className={styles.colAtt}>Attempts</th>
            <th className={styles.colActions}></th>
          </tr>
        </thead>
        <tbody>
          {jobs.length === 0 && !loading && (
            <tr><td colSpan={7} className={styles.empty}>No failed jobs. 🎉</td></tr>
          )}
          {jobs.map(job => (
            <tr key={job.id}>
              <td className={`${styles.colTime} label-sm`}>
                {new Date(job.created_at).toLocaleString(undefined, {
                  month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
                })}
              </td>
              <td className={`${styles.colSvc} label-sm`}>{job.service}</td>
              <td className={`${styles.colType} label-sm`}>{job.media_type || '—'}</td>
              <td className={`${styles.colPath} body-sm`} title={job.source_path}>{job.source_path || '—'}</td>
              <td className={`${styles.colReason} body-sm`} title={job.reason}>{job.reason || '—'}</td>
              <td className={`${styles.colAtt} label-sm`}>{job.attempts}</td>
              <td className={styles.colActions}>
                <div className={styles.actions}>
                  <button
                    className="btn btn-icon"
                    onClick={() => void retry(job.id)}
                    disabled={acting === job.id}
                    aria-label="Retry"
                    title="Retry"
                  >
                    <RiRestartLine size={16} />
                  </button>
                  <button
                    className={`btn btn-icon ${styles.dismissBtn}`}
                    onClick={() => void dismiss(job.id)}
                    disabled={acting === job.id}
                    aria-label="Dismiss"
                    title="Dismiss"
                  >
                    <RiCloseLine size={16} />
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className={styles.pagination}>
        <span className={`label-sm ${styles.pageInfo}`}>
          {total} {total === 1 ? 'job' : 'jobs'}
        </span>
        <div style={{ display: 'flex', gap: 'var(--space-1)' }}>
          <button className="btn" onClick={() => goToPage(page - 1)} disabled={page <= 1}>Previous</button>
          <span className={`label-sm ${styles.pageInfo}`} style={{ alignSelf: 'center', padding: '0 var(--space-2)' }}>
            {page} / {totalPages}
          </span>
          <button className="btn" onClick={() => goToPage(page + 1)} disabled={page >= totalPages}>Next</button>
        </div>
      </div>
    </div>
  )
}

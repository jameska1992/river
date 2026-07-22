import { useEffect, useState } from 'react'
import { RiFilmLine, RiTv2Line, RiMusicLine, RiHeadphoneLine, RiFolder3Line, RiScanLine, RiCheckLine, RiUserLine, RiRefreshLine } from 'react-icons/ri'
import { useLibraries } from '../../context/LibrariesContext'
import { api, ApiError } from '../../api'
import type { LibraryType, ActiveSession } from '../../api'
import styles from './OverviewPage.module.css'

const typeConfig: Record<LibraryType, { label: string; icon: React.ReactNode; color: string }> = {
  movie:     { label: 'Movies',     icon: <RiFilmLine />,      color: 'var(--color-primary)' },
  tvshow:    { label: 'TV Shows',   icon: <RiTv2Line />,       color: 'var(--color-secondary)' },
  music:     { label: 'Music',      icon: <RiMusicLine />,     color: 'var(--color-tertiary)' },
  audiobook: { label: 'Audiobooks', icon: <RiHeadphoneLine />, color: 'var(--color-primary)' },
}

type ScanState = 'idle' | 'loading' | 'done' | 'error'

interface Stats { movies: number; tv_shows: number; tracks: number; audiobooks: number }

export function OverviewPage() {
  const { libraries, isLoading, fetch } = useLibraries()
  const [scanState, setScanState] = useState<ScanState>('idle')
  const [scanError, setScanError] = useState('')
  const [stats, setStats] = useState<Stats | null>(null)
  const [activeSessions, setActiveSessions] = useState<ActiveSession[]>([])

  useEffect(() => {
    void fetch()
    api.getStats().then(setStats).catch(() => {})
  }, [fetch])

  useEffect(() => {
    const load = () => api.getActiveSessions().then(setActiveSessions).catch(() => {})
    load()
    const timer = setInterval(load, 30_000)
    return () => clearInterval(timer)
  }, [])

  async function handleScan() {
    setScanState('loading')
    setScanError('')
    try {
      await api.triggerScan()
      setScanState('done')
      setTimeout(() => setScanState('idle'), 3000)
    } catch (err) {
      setScanError(err instanceof ApiError ? err.message : 'Failed to trigger scan.')
      setScanState('error')
    }
  }

  const itemCounts: Record<LibraryType, number | null> = {
    movie:     stats?.movies     ?? null,
    tvshow:    stats?.tv_shows   ?? null,
    music:     stats?.tracks     ?? null,
    audiobook: stats?.audiobooks ?? null,
  }

  const itemLabels: Record<LibraryType, string> = {
    movie:     'movies',
    tvshow:    'shows',
    music:     'tracks',
    audiobook: 'audiobooks',
  }

  const formatProgress = (position: number, duration: number) => {
    const fmt = (s: number) => {
      const h = Math.floor(s / 3600)
      const m = Math.floor((s % 3600) / 60)
      const sec = Math.floor(s % 60)
      if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
      return `${m}:${String(sec).padStart(2, '0')}`
    }
    return `${fmt(position)} / ${fmt(duration)}`
  }

  const timeAgo = (iso: string) => {
    const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
    if (secs < 60) return `${secs}s ago`
    const mins = Math.floor(secs / 60)
    if (mins < 60) return `${mins}m ago`
    return `${Math.floor(mins / 60)}h ago`
  }

  const byType = (Object.keys(typeConfig) as LibraryType[]).map(type => ({
    type,
    ...typeConfig[type],
    count:      libraries.filter(l => l.type === type).length,
    itemCount:  itemCounts[type],
    itemLabel:  itemLabels[type],
  }))

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-lg ${styles.heading}`}>Overview</h1>
        <div className={styles.scanWrap}>
          {scanState === 'error' && (
            <span className={styles.scanError}>{scanError}</span>
          )}
          <button
            className={`btn ${scanState === 'done' ? styles.btnDone : 'btn-secondary'}`}
            onClick={handleScan}
            disabled={scanState === 'loading' || scanState === 'done'}
          >
            {scanState === 'loading' && <RiScanLine size={16} className={styles.spinning} />}
            {scanState === 'done'    && <RiCheckLine size={16} />}
            {scanState === 'idle'    && <RiScanLine size={16} />}
            {scanState === 'error'   && <RiScanLine size={16} />}
            {scanState === 'loading' ? 'Starting…'   :
             scanState === 'done'    ? 'Scan started' :
             'Scan Now'}
          </button>
        </div>
      </div>

      {isLoading ? (
        <p className="label-sm">Loading…</p>
      ) : (
        <div className={styles.grid}>
          <div className={`surface-low ${styles.card}`}>
            <div className={styles.cardIcon} style={{ color: 'var(--color-on-surface-variant)' }}>
              <RiFolder3Line />
            </div>
            <span className={`label-sm ${styles.cardLabel}`}>Total Libraries</span>
            <span className={`headline-md ${styles.cardValue}`}>{libraries.length}</span>
          </div>

          {byType.map(({ type, label, icon, color, count, itemCount, itemLabel }) => (
            <div key={type} className={`surface-low ${styles.card}`}>
              <div className={styles.cardIcon} style={{ color }}>
                {icon}
              </div>
              <span className={`label-sm ${styles.cardLabel}`}>{label}</span>
              <span className={`headline-md ${styles.cardValue}`}>{count}</span>
              {itemCount !== null && (
                <span className={`label-sm ${styles.cardSub}`}>
                  {itemCount.toLocaleString()} {itemLabel}
                </span>
              )}
            </div>
          ))}
        </div>
      )}

      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={`headline-md ${styles.subheading}`}>Now Watching</h2>
          <span className={`label-sm ${styles.sessionCount}`}>
            {activeSessions.length === 0
              ? 'No one active'
              : `${activeSessions.length} active`}
          </span>
        </div>
        {activeSessions.length === 0 ? (
          <div className={styles.emptySession}>
            <RiUserLine size={20} />
            <span className="body-sm">No one is watching right now</span>
          </div>
        ) : (
          <div className={styles.list}>
            {activeSessions.map((s, i) => {
              const pct = s.duration > 0 ? (s.position / s.duration) * 100 : 0
              return (
                <div key={`${s.user_id}-${i}`} className={`surface ${styles.sessionItem}`}>
                  <div className={styles.sessionAvatar}>
                    {s.username[0].toUpperCase()}
                  </div>
                  <div className={styles.sessionMeta}>
                    <div className={styles.sessionTop}>
                      <span className={`label-md ${styles.sessionUser}`}>{s.username}</span>
                      <span className={`label-sm ${styles.sessionTime}`}>
                        <RiRefreshLine size={11} />
                        {timeAgo(s.updated_at)}
                      </span>
                    </div>
                    <span className={`label-sm ${styles.sessionTitle}`}>
                      {s.show_title ? `${s.show_title} · ${s.title}` : s.title}
                    </span>
                    <div className={styles.sessionProgressRow}>
                      <div className={styles.sessionProgressBar}>
                        <div className={styles.sessionProgressFill} style={{ width: `${pct}%` }} />
                      </div>
                      <span className={`label-sm ${styles.sessionPos}`}>
                        {formatProgress(s.position, s.duration)}
                      </span>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {!isLoading && libraries.length > 0 && (
        <div className={styles.section}>
          <h2 className={`headline-md ${styles.subheading}`}>Libraries</h2>
          <div className={styles.list}>
            {libraries.map(lib => {
              const cfg = typeConfig[lib.type]
              return (
                <div key={lib.id} className={`surface ${styles.listItem}`}>
                  <span className={styles.listIcon} style={{ color: cfg.color }}>
                    {cfg.icon}
                  </span>
                  <div className={styles.listMeta}>
                    <span className="label-md">{lib.name}</span>
                    <span className="label-sm">{cfg.label}</span>
                  </div>
                  <span className={`badge badge-primary ${styles.pathCount}`}>
                    {lib.paths.length} {lib.paths.length === 1 ? 'path' : 'paths'}
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

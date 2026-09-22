import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiTimeLine, RiCheckboxCircleLine, RiPlayCircleLine, RiUserLine, RiHardDrive2Line, RiErrorWarningLine, RiFilmLine, RiTv2Line, RiMusicLine, RiHeadphoneLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { WatchInsights, LibraryInsights, LibraryType } from '../../api'
import styles from './InsightsPage.module.css'

const WINDOWS: { value: string; label: string }[] = [
  { value: '7d',  label: '7 days' },
  { value: '30d', label: '30 days' },
  { value: '90d', label: '90 days' },
  { value: 'all', label: 'All time' },
]

const libTypeIcon: Record<LibraryType, React.ReactNode> = {
  movie:     <RiFilmLine />,
  tvshow:    <RiTv2Line />,
  music:     <RiMusicLine />,
  audiobook: <RiHeadphoneLine />,
}

// formatBytes renders a byte count as a compact "4.2 GB" style string.
function formatBytes(n: number): string {
  if (n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / Math.pow(1024, i)
  return `${i === 0 || v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

// formatDuration renders a second count as a compact "3h 42m" / "12m" string.
function formatDuration(totalSeconds: number): string {
  const s = Math.floor(totalSeconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`
  if (m > 0) return `${m}m`
  return `${s}s`
}

function formatDay(iso: string): string {
  // iso is "YYYY-MM-DD" — parse as UTC to avoid a local-timezone off-by-one.
  const d = new Date(`${iso}T00:00:00Z`)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' })
}

export function InsightsPage() {
  const [window, setWindow] = useState('30d')
  const [data, setData] = useState<WatchInsights | null>(null)
  const [library, setLibrary] = useState<LibraryInsights | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Library health is independent of the watch window — fetch it once.
  useEffect(() => {
    api.getLibraryInsights().then(setLibrary).catch(() => {})
  }, [])

  const load = useCallback(async (w: string) => {
    setLoading(true)
    setError('')
    try {
      setData(await api.getWatchInsights(w))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load insights.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- refetches (and toggles loading) when the selected window changes
    void load(window)
  }, [window, load])

  const maxActivity = data ? Math.max(1, ...data.activity.map(d => d.watch_seconds)) : 1
  const hasActivity = !!data && data.started > 0

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1 className={`headline-lg ${styles.heading}`}>Insights</h1>
        <div className={styles.windowTabs} role="group" aria-label="Time window">
          {WINDOWS.map(w => (
            <button
              key={w.value}
              className={`${styles.windowTab} ${window === w.value ? styles.windowTabActive : ''}`}
              onClick={() => setWindow(w.value)}
            >
              {w.label}
            </button>
          ))}
        </div>
      </div>

      {error && <p className={styles.error}>{error}</p>}
      {loading && !data && <p className="label-sm">Loading…</p>}

      {data && (
        <>
          {/* Headline tiles */}
          <div className={styles.grid}>
            <Tile icon={<RiTimeLine />} color="var(--color-primary)"
              label="Total watch time" value={formatDuration(data.total_watch_seconds)} />
            <Tile icon={<RiCheckboxCircleLine />} color="var(--color-secondary)"
              label="Completion rate" value={`${Math.round(data.completion_rate * 100)}%`}
              sub={`${data.completed.toLocaleString()} of ${data.started.toLocaleString()} finished`} />
            <Tile icon={<RiPlayCircleLine />} color="var(--color-tertiary)"
              label="In-progress items" value={data.started.toLocaleString()} />
            <Tile icon={<RiUserLine />} color="var(--color-primary)"
              label="Active viewers" value={data.per_user.length.toLocaleString()} />
          </div>

          {!hasActivity ? (
            <div className={styles.empty}>
              <RiPlayCircleLine size={22} />
              <span className="body-sm">No watch activity in this window yet.</span>
            </div>
          ) : (
            <>
              {/* Activity over time */}
              <section className={styles.section}>
                <h2 className={`headline-md ${styles.subheading}`}>Activity over time</h2>
                {data.activity.length === 0 ? (
                  <p className="label-sm">No dated activity.</p>
                ) : (
                  <div className={`surface-low ${styles.chartCard}`}>
                    <div className={styles.chart}>
                      {data.activity.map(d => (
                        <div key={d.date} className={styles.barWrap} title={`${formatDay(d.date)} · ${formatDuration(d.watch_seconds)} · ${d.plays} plays`}>
                          <div className={styles.bar} style={{ height: `${Math.max(2, (d.watch_seconds / maxActivity) * 100)}%` }} />
                        </div>
                      ))}
                    </div>
                    <div className={styles.chartAxis}>
                      <span className="label-sm">{formatDay(data.activity[0].date)}</span>
                      <span className="label-sm">{formatDay(data.activity[data.activity.length - 1].date)}</span>
                    </div>
                  </div>
                )}
              </section>

              <div className={styles.twoCol}>
                {/* Most watched */}
                <section className={styles.section}>
                  <h2 className={`headline-md ${styles.subheading}`}>Most watched</h2>
                  {data.top_titles.length === 0 ? (
                    <p className="label-sm">Nothing yet.</p>
                  ) : (
                    <div className={styles.list}>
                      {data.top_titles.map((t, i) => (
                        <div key={`${t.media_type}-${t.media_id}`} className={`surface ${styles.topItem}`}>
                          <span className={styles.rank}>{i + 1}</span>
                          <div className={styles.topMeta}>
                            <span className={`label-md ${styles.topTitle}`}>
                              {t.show_title ? `${t.show_title} · ${t.title}` : t.title}
                            </span>
                            <span className={`label-sm ${styles.topSub}`}>
                              {t.plays.toLocaleString()} {t.plays === 1 ? 'play' : 'plays'} · {t.completions.toLocaleString()} completed
                            </span>
                          </div>
                          <span className={`label-sm ${styles.topTime}`}>{formatDuration(t.watch_seconds)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </section>

                {/* Per-user */}
                <section className={styles.section}>
                  <h2 className={`headline-md ${styles.subheading}`}>By viewer</h2>
                  {data.per_user.length === 0 ? (
                    <p className="label-sm">Nothing yet.</p>
                  ) : (
                    <div className={styles.list}>
                      {data.per_user.map(u => (
                        <div key={u.user_id} className={`surface ${styles.userItem}`}>
                          <div className={styles.userAvatar}>{(u.username || '?')[0].toUpperCase()}</div>
                          <div className={styles.topMeta}>
                            <span className={`label-md ${styles.topTitle}`}>{u.username || 'Unknown user'}</span>
                            <span className={`label-sm ${styles.topSub}`}>
                              {u.item_count.toLocaleString()} {u.item_count === 1 ? 'item' : 'items'}
                            </span>
                          </div>
                          <span className={`label-sm ${styles.topTime}`}>{formatDuration(u.watch_seconds)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </section>
              </div>
            </>
          )}
        </>
      )}

      {library && (
        <section className={styles.section}>
          <div className={styles.libHeaderRow}>
            <h2 className={`headline-md ${styles.subheading}`}>Library health</h2>
            <div className={styles.libSummary}>
              <span className={`badge ${styles.storageBadge}`}>
                <RiHardDrive2Line size={13} /> {formatBytes(library.total_size_bytes)}
              </span>
              <Link
                to="/admin/failed-jobs"
                className={`badge ${library.failed_jobs > 0 ? styles.failedBadge : ''}`}
              >
                <RiErrorWarningLine size={13} /> {library.failed_jobs.toLocaleString()} failed
              </Link>
            </div>
          </div>

          {library.libraries.length === 0 ? (
            <p className="label-sm">No libraries configured.</p>
          ) : (
            <div className={styles.libTable}>
              <div className={`label-sm ${styles.libHead}`}>
                <span>Library</span>
                <span className={styles.libNum}>Items</span>
                <span className={styles.libNum}>Size</span>
                <span className={styles.libNum}>Untranscoded</span>
              </div>
              {library.libraries.map(l => (
                <div key={l.id} className={`surface ${styles.libRow}`}>
                  <div className={styles.libName}>
                    <span className={styles.libIcon} aria-hidden>{libTypeIcon[l.type]}</span>
                    <span className={`label-md ${styles.topTitle}`}>{l.name}</span>
                  </div>
                  <span className={`label-sm ${styles.libNum}`}>{l.item_count.toLocaleString()}</span>
                  <span className={`label-sm ${styles.libNum}`}>{formatBytes(l.size_bytes)}</span>
                  <span className={`label-sm ${styles.libNum} ${l.untranscoded > 0 ? styles.warn : ''}`}>
                    {l.untranscoded.toLocaleString()}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  )
}

function Tile({ icon, color, label, value, sub }: {
  icon: React.ReactNode; color: string; label: string; value: string; sub?: string
}) {
  return (
    <div className={`surface-low ${styles.card}`}>
      <div className={styles.cardIcon} style={{ color }}>{icon}</div>
      <span className={`label-sm ${styles.cardLabel}`}>{label}</span>
      <span className={`headline-md ${styles.cardValue}`}>{value}</span>
      {sub && <span className={`label-sm ${styles.cardSub}`}>{sub}</span>}
    </div>
  )
}

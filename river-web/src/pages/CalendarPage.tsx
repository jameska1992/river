import { useEffect, useMemo, useState } from 'react'
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiFilmLine,
  RiTv2Line,
  RiLoaderLine,
} from 'react-icons/ri'
import { api, ApiError } from '../api'
import type { CalendarItem } from '../api'
import styles from './CalendarPage.module.css'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]

// Local YYYY-MM-DD key (not UTC) so events group onto the day the user sees.
function ymd(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

function episodeCode(it: CalendarItem): string {
  return `S${pad2(it.seasonNumber ?? 0)}E${pad2(it.episodeNumber ?? 0)}`
}

function eventLabel(it: CalendarItem): string {
  return it.type === 'episode' ? `${it.title} · ${episodeCode(it)}` : it.title
}

// Download state, shown as a right-hand border on each entry:
//   downloaded — file is present (grabbed + imported) → green
//   missing    — release/air date has passed but no file yet → red
//   upcoming   — not yet released/aired, so not ready to download → blue
type EventState = 'downloaded' | 'missing' | 'upcoming'

function eventState(it: CalendarItem): EventState {
  if (it.hasFile) return 'downloaded'
  // A movie isn't downloadable until its digital release, even if it's already
  // in cinemas — base availability on that date when Radarr knows it.
  const availableDate = it.digitalRelease || it.date
  return new Date(availableDate).getTime() <= Date.now() ? 'missing' : 'upcoming'
}

const stateLabel: Record<EventState, string> = {
  downloaded: 'Downloaded',
  missing: 'Missing',
  upcoming: 'Not yet available',
}

function eventTooltip(it: CalendarItem): string {
  const status = stateLabel[eventState(it)]
  if (it.type === 'episode') {
    return [`${it.title} — ${episodeCode(it)}`, it.episodeTitle, status, it.overview]
      .filter(Boolean).join('\n')
  }
  const rel = it.releaseType
    ? `${it.releaseType[0].toUpperCase()}${it.releaseType.slice(1)} release`
    : ''
  return [it.title, rel, status, it.overview].filter(Boolean).join('\n')
}

export function CalendarPage() {
  // First of the currently-viewed month.
  const [viewDate, setViewDate] = useState(() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), 1)
  })
  const [items, setItems] = useState<CalendarItem[]>([])
  const [loading, setLoading] = useState(false)
  const [notConfigured, setNotConfigured] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Fixed 6-week (42-day) grid starting on the Sunday on/before the 1st.
  const days = useMemo(() => {
    const gridStart = new Date(viewDate)
    gridStart.setDate(1 - viewDate.getDay())
    return Array.from({ length: 42 }, (_, i) => {
      const d = new Date(gridStart)
      d.setDate(gridStart.getDate() + i)
      return d
    })
  }, [viewDate])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError(null)
      setNotConfigured(false)
      try {
        const data = await api.getCalendar(ymd(days[0]), ymd(days[days.length - 1]))
        if (!cancelled) setItems(data)
      } catch (err) {
        if (cancelled) return
        if (err instanceof ApiError && err.status === 503) {
          setNotConfigured(true)
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load calendar')
        }
        setItems([])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [days])

  const byDay = useMemo(() => {
    const map = new Map<string, CalendarItem[]>()
    for (const it of items) {
      const key = ymd(new Date(it.date))
      const arr = map.get(key)
      if (arr) arr.push(it)
      else map.set(key, [it])
    }
    return map
  }, [items])

  const todayKey = ymd(new Date())

  function prevMonth() { setViewDate(d => new Date(d.getFullYear(), d.getMonth() - 1, 1)) }
  function nextMonth() { setViewDate(d => new Date(d.getFullYear(), d.getMonth() + 1, 1)) }
  function goToday() {
    const n = new Date()
    setViewDate(new Date(n.getFullYear(), n.getMonth(), 1))
  }

  return (
    <div className="container" style={{ paddingTop: 'var(--space-5)', paddingBottom: 'var(--space-5)' }}>
      <div className={styles.header}>
        <h1 className={`headline-sm ${styles.heading}`}>Calendar</h1>
        <div className={styles.controls}>
          {loading && <RiLoaderLine className={styles.spinner} aria-label="Loading" />}
          <button className="btn btn-sm" onClick={goToday}>Today</button>
          <button className={styles.navBtn} onClick={prevMonth} aria-label="Previous month">
            <RiArrowLeftSLine />
          </button>
          <span className={styles.monthLabel}>
            {MONTHS[viewDate.getMonth()]} {viewDate.getFullYear()}
          </span>
          <button className={styles.navBtn} onClick={nextMonth} aria-label="Next month">
            <RiArrowRightSLine />
          </button>
        </div>
      </div>

      <div className={styles.legend}>
        <span className={styles.legendItem}>
          <span className={`${styles.legendDot} ${styles.eventMovie}`} /> Movies
        </span>
        <span className={styles.legendItem}>
          <span className={`${styles.legendDot} ${styles.eventEpisode}`} /> Episodes
        </span>
      </div>

      {notConfigured ? (
        <div className={styles.status}>
          Neither Radarr nor Sonarr is configured. Set{' '}
          <code>RADARR_URL / RADARR_API_KEY</code> or{' '}
          <code>SONARR_URL / SONARR_API_KEY</code> in river-api.
        </div>
      ) : error ? (
        <div className={`${styles.status} ${styles.statusError}`}>{error}</div>
      ) : (
        <div className={styles.gridScroll}>
          <div className={styles.weekdays}>
            {WEEKDAYS.map(w => <div key={w} className={styles.weekday}>{w}</div>)}
          </div>
          <div className={styles.grid}>
            {days.map(day => {
              const key = ymd(day)
              const dayItems = byDay.get(key) ?? []
              const inMonth = day.getMonth() === viewDate.getMonth()
              const isToday = key === todayKey
              return (
                <div
                  key={key}
                  className={`${styles.cell} ${inMonth ? '' : styles.cellMuted} ${isToday ? styles.cellToday : ''}`}
                >
                  <div className={styles.cellDate}>{day.getDate()}</div>
                  <div className={styles.cellEvents}>
                    {dayItems.map((it, i) => (
                      <div
                        key={i}
                        className={`${styles.event} ${it.type === 'movie' ? styles.eventMovie : styles.eventEpisode} ${styles[eventState(it)]}`}
                        title={eventTooltip(it)}
                      >
                        {it.type === 'movie'
                          ? <RiFilmLine className={styles.eventIcon} />
                          : <RiTv2Line className={styles.eventIcon} />}
                        <span className={styles.eventLabel}>{eventLabel(it)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

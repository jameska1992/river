import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../api'
import type { AudioTrack, Movie, Subtitle } from '../../api'
import { Player, type PlayerEngineInfo } from '../../player/react/Player'
import { DebugPanel } from '../../player/react/DebugPanel'
import { movieSource } from '../../player/riverSource'
import type { EnginePreference } from '../../player/core/selectEngine'

const ENGINES: EnginePreference[] = ['auto', 'mediabunny', 'native']

/**
 * /debug/mediabunny-player — exercises the prototype player subsystem against
 * real River movies. Admin-only and not linked from the UI; the production
 * watch pages are unaffected.
 *
 * Query params: ?movie=<id>&engine=auto|mediabunny|native
 */
export function MediabunnyPlayerPage() {
  const [params, setParams] = useSearchParams()
  const movieId = params.get('movie') ?? ''
  const preference = (ENGINES.find(e => e === params.get('engine')) ?? 'auto') as EnginePreference

  const [movies, setMovies] = useState<Movie[]>([])
  const [audioTracks, setAudioTracks] = useState<AudioTrack[] | null>(null)
  const [subtitles, setSubtitles] = useState<Subtitle[]>([])
  const [subtitleId, setSubtitleId] = useState('')
  const [info, setInfo] = useState<PlayerEngineInfo | null>(null)
  const [log, setLog] = useState<string[]>([])

  const addLog = useCallback((line: string) => {
    setLog(l => [`${new Date().toLocaleTimeString()}  ${line}`, ...l].slice(0, 50))
  }, [])

  useEffect(() => {
    api.listMovies({ limit: 500, sort: 'title', order: 'asc' }).then(setMovies).catch(err => addLog(`list movies: ${err}`))
  }, [addLog])

  useEffect(() => {
    if (!movieId) return
    let stale = false
    Promise.all([
      api.getMovieAudioTracks(movieId).catch(() => [] as AudioTrack[]),
      api.getMovieSubtitles(movieId).catch(() => [] as Subtitle[]),
    ]).then(([a, s]) => {
      if (stale) return
      setAudioTracks(a)
      setSubtitles(s)
      setSubtitleId('')
    })
    return () => { stale = true; setAudioTracks(null) }
  }, [movieId])

  const movie = movies.find(m => m.id === movieId)
  // Built only once the audio-track list is known so the player loads once per movie.
  const source = useMemo(
    () => (movieId && audioTracks ? movieSource(api, movieId, movie?.title, audioTracks) : null),
    // movie?.title is display-only; don't reload when the movie list arrives.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [movieId, audioTracks],
  )

  const onEngine = useCallback((i: PlayerEngineInfo | null) => {
    setInfo(i)
    // Handle for driving/inspecting the engine from devtools and browser automation.
    ;(window as unknown as { __riverPlayer?: unknown }).__riverPlayer = i?.engine
    if (i) addLog(`engine: ${i.engine.kind} — ${i.reason}`)
  }, [addLog])

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  // Scripted seek patterns for exercising the seek path.
  const seekTest = async (kind: 'start' | 'middle' | 'nearEnd' | 'burst') => {
    const e = info?.engine
    if (!e) return
    const d = e.getState().duration
    const t0 = performance.now()
    if (kind === 'start') await e.seek(0)
    if (kind === 'middle') await e.seek(d / 2)
    if (kind === 'nearEnd') await e.seek(Math.max(0, d - 5))
    if (kind === 'burst') {
      const targets = Array.from({ length: 8 }, () => Math.random() * d)
      // Fire without awaiting — only the last should win.
      const all = targets.map(t => e.seek(t))
      await Promise.all(all)
      addLog(`burst targets: ${targets.map(t => t.toFixed(0)).join(', ')} → landed ${e.getState().position.toFixed(1)}s`)
    }
    addLog(`seek ${kind}: ${(performance.now() - t0).toFixed(0)} ms → ${e.getState().position.toFixed(2)}s`)
  }

  return (
    <div className="container" style={{ paddingTop: 'var(--space-5)', paddingBottom: 'var(--space-6)' }}>
      <h1 className="headline-md">Mediabunny Player <span className="label-sm">prototype</span></h1>

      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', margin: '16px 0' }}>
        <label>
          Movie{' '}
          <select value={movieId} onChange={e => setParam('movie', e.target.value)}>
            <option value="">— choose —</option>
            {movies.map(m => <option key={m.id} value={m.id}>{m.title}{m.year ? ` (${m.year})` : ''}</option>)}
          </select>
        </label>
        <label>
          Engine{' '}
          <select value={preference} onChange={e => setParam('engine', e.target.value)}>
            {ENGINES.map(e => <option key={e} value={e}>{e}</option>)}
          </select>
        </label>
        <label>
          Subtitles{' '}
          <select value={subtitleId} onChange={e => setSubtitleId(e.target.value)} disabled={subtitles.length === 0}>
            <option value="">Off</option>
            {subtitles.map(s => <option key={s.id} value={s.id}>{s.label || s.language}</option>)}
          </select>
        </label>
      </div>

      <Player
        source={source}
        preference={preference}
        subtitleUrl={subtitleId ? api.subtitleStreamUrl(subtitleId) : null}
        onEngine={onEngine}
        onRecover={r => addLog(`recovery: ${r}`)}
      />

      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '12px 0' }}>
        <button className="btn btn-secondary" onClick={() => void seekTest('start')} disabled={!info}>Seek 0</button>
        <button className="btn btn-secondary" onClick={() => void seekTest('middle')} disabled={!info}>Seek 50%</button>
        <button className="btn btn-secondary" onClick={() => void seekTest('nearEnd')} disabled={!info}>Seek end−5s</button>
        <button className="btn btn-secondary" onClick={() => void seekTest('burst')} disabled={!info}>Seek burst ×8</button>
        <button className="btn btn-secondary" onClick={() => { addLog('manual reload'); void info?.engine.reload() }} disabled={!info}>Reload pipeline</button>
      </div>

      <div style={{ display: 'grid', gap: 16, gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))' }}>
        <DebugPanel engine={info?.engine ?? null} reason={info?.reason} />
        <pre style={{ margin: 0, fontSize: 12, maxHeight: 360, overflow: 'auto' }}>{log.join('\n') || 'Event log'}</pre>
      </div>
    </div>
  )
}

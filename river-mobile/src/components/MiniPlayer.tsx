import { RiPlayFill, RiPauseFill, RiSkipForwardFill, RiMusic2Line } from 'react-icons/ri'
import { useAudioPlayer } from '../context/audioPlayerContext'
import { imageUrl } from '../util/imageUrl'
import { progressPct } from '../util/format'

// Docked player bar that sits directly above the bottom tabs whenever
// something is loaded. Tapping the artwork / text expands the full player;
// the buttons are play/pause and next. A thin progress line rides the top.
export function MiniPlayer() {
  const { current, playing, position, duration, toggle, next, expand } = useAudioPlayer()
  if (!current) return null

  const pct = progressPct(position, duration)
  const art = imageUrl(current.artworkPath, 'poster')

  return (
    <div style={styles.wrap} role="region" aria-label="Now playing">
      <div style={styles.progress}><div style={{ ...styles.progressFill, width: `${pct}%` }} /></div>
      <div style={styles.row}>
        <button style={styles.main} onClick={expand} aria-label="Open player">
          {art
            ? <img src={art} alt="" style={styles.art} />
            : <span style={styles.artFallback}><RiMusic2Line /></span>}
          <span style={styles.text}>
            <span style={styles.title}>{current.title}</span>
            <span style={styles.subtitle}>{current.artist || current.album}</span>
          </span>
        </button>
        <button style={styles.ctrl} aria-label={playing ? 'Pause' : 'Play'} onClick={toggle}>
          {playing ? <RiPauseFill /> : <RiPlayFill />}
        </button>
        <button style={styles.ctrl} aria-label="Next" onClick={next}>
          <RiSkipForwardFill />
        </button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    position: 'fixed',
    left: 0, right: 0,
    bottom: 'calc(var(--tabbar-h) + var(--safe-bottom))',
    height: '3.75rem',
    background: 'var(--bg-elev-2)',
    borderTop: '1px solid var(--outline)',
    zIndex: 45,
    display: 'flex', flexDirection: 'column',
  },
  progress: { height: '2px', background: 'transparent' },
  progressFill: { height: '100%', background: 'var(--accent)' },
  row: { flex: 1, display: 'flex', alignItems: 'center', minWidth: 0 },
  main: {
    flex: 1, minWidth: 0,
    display: 'flex', alignItems: 'center', gap: '0.75rem',
    padding: '0 0.5rem 0 0.6rem', background: 'transparent', textAlign: 'left', height: '100%',
  },
  art: { width: '2.75rem', height: '2.75rem', borderRadius: 'var(--radius-sm)', objectFit: 'cover', flex: '0 0 auto' },
  artFallback: {
    width: '2.75rem', height: '2.75rem', borderRadius: 'var(--radius-sm)', flex: '0 0 auto',
    display: 'grid', placeItems: 'center', background: 'var(--bg-elev-3)', color: 'var(--text-muted)',
  },
  text: { display: 'flex', flexDirection: 'column', minWidth: 0 },
  title: { fontSize: '0.9rem', fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  subtitle: { fontSize: '0.75rem', color: 'var(--text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  ctrl: {
    width: '3rem', height: '100%', flex: '0 0 auto',
    display: 'grid', placeItems: 'center', fontSize: '1.4rem', color: 'var(--text)', background: 'transparent',
  },
}

import { useRef, useState } from 'react'
import {
  RiArrowDownSLine, RiPlayFill, RiPauseFill,
  RiReplay10Line, RiForward10Line,
  RiSkipBackFill, RiSkipForwardFill,
  RiPlayListLine, RiMusic2Line,
} from 'react-icons/ri'
import { useAudioPlayer } from '../context/audioPlayerContext'
import { imageUrl } from '../util/imageUrl'
import { formatDuration } from '../util/format'
import { seekFraction } from '../util/player'
import { BottomSheet, SheetOption } from './BottomSheet'

const SEEK_SECONDS = 10

// The expanded player: large artwork, scrub bar, transport, and a queue /
// chapter-list sheet. Rendered by AudioPlayerProvider while `expanded`.
export function FullPlayer() {
  const {
    current, queue, index, playing, position, duration,
    toggle, seek, skip, next, prev, jumpTo, collapse,
  } = useAudioPlayer()
  const barRef = useRef<HTMLDivElement>(null)
  const [scrubFrac, setScrubFrac] = useState<number | null>(null)
  const [showQueue, setShowQueue] = useState(false)

  if (!current) return null

  const displayed = scrubFrac != null ? scrubFrac * duration : position
  const pct = duration > 0 ? (displayed / duration) * 100 : 0
  const remaining = Math.max(0, duration - displayed)
  const art = imageUrl(current.artworkPath, 'poster')
  const isBook = current.progressKind === 'chapter'

  const scrubTo = (clientX: number) => {
    const el = barRef.current
    if (!el || !duration) return
    const r = el.getBoundingClientRect()
    const frac = seekFraction(clientX, r.left, r.width)
    setScrubFrac(frac)
    seek(frac * duration)
  }

  return (
    <div style={styles.page}>
      <div style={styles.header}>
        <button aria-label="Minimize player" onClick={collapse} style={styles.headerBtn}><RiArrowDownSLine /></button>
        <span style={styles.headerLabel}>{current.album}</span>
        <button aria-label={isBook ? 'Chapters' : 'Queue'} onClick={() => setShowQueue(true)} style={styles.headerBtn}>
          <RiPlayListLine />
        </button>
      </div>

      <div style={styles.artWrap}>
        {art
          ? <img src={art} alt="" style={styles.art} />
          : <div style={styles.artFallback}><RiMusic2Line /></div>}
      </div>

      <div style={styles.meta}>
        <div style={styles.title}>{current.title}</div>
        <div style={styles.subtitle}>{current.artist || current.album}</div>
      </div>

      <div style={styles.scrubWrap}>
        <div
          ref={barRef}
          role="slider"
          aria-label="Seek"
          aria-valuemin={0}
          aria-valuemax={Math.floor(duration)}
          aria-valuenow={Math.floor(displayed)}
          style={styles.barHit}
          onPointerDown={e => { (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId); scrubTo(e.clientX) }}
          onPointerMove={e => { if (scrubFrac != null) scrubTo(e.clientX) }}
          onPointerUp={() => setScrubFrac(null)}
          onPointerCancel={() => setScrubFrac(null)}
        >
          <div style={styles.barTrack}>
            <div style={{ ...styles.barFill, width: `${pct}%` }} />
            <div style={{ ...styles.barThumb, left: `${pct}%` }} />
          </div>
        </div>
        <div style={styles.timeRow}>
          <span>{formatDuration(Math.floor(displayed))}</span>
          <span>-{formatDuration(Math.ceil(remaining))}</span>
        </div>
      </div>

      <div style={styles.transport}>
        <button aria-label="Previous" onClick={prev} style={styles.sideBtn}><RiSkipBackFill /></button>
        <button aria-label={`Back ${SEEK_SECONDS} seconds`} onClick={() => skip(-SEEK_SECONDS)} style={styles.sideBtn}><RiReplay10Line /></button>
        <button aria-label={playing ? 'Pause' : 'Play'} onClick={toggle} style={styles.playBtn}>
          {playing ? <RiPauseFill /> : <RiPlayFill />}
        </button>
        <button aria-label={`Forward ${SEEK_SECONDS} seconds`} onClick={() => skip(SEEK_SECONDS)} style={styles.sideBtn}><RiForward10Line /></button>
        <button aria-label="Next" onClick={next} style={styles.sideBtn}><RiSkipForwardFill /></button>
      </div>

      {showQueue && (
        <BottomSheet title={isBook ? 'Chapters' : 'Up next'} onClose={() => setShowQueue(false)}>
          {queue.map((item, i) => (
            <SheetOption
              key={item.id}
              label={item.title}
              active={i === index}
              onSelect={() => { jumpTo(i); setShowQueue(false) }}
            />
          ))}
        </BottomSheet>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    position: 'fixed', inset: 0, zIndex: 60,
    background: 'var(--bg)',
    display: 'flex', flexDirection: 'column',
    padding: 'calc(0.5rem + env(safe-area-inset-top,0px)) 1.5rem calc(1.5rem + env(safe-area-inset-bottom,0px))',
    gap: '1rem',
  },
  header: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem' },
  headerBtn: { width: '2.75rem', height: '2.75rem', display: 'grid', placeItems: 'center', fontSize: '1.6rem', color: 'var(--text)', background: 'transparent' },
  headerLabel: { flex: 1, textAlign: 'center', fontSize: '0.85rem', color: 'var(--text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },

  artWrap: { flex: 1, display: 'grid', placeItems: 'center', minHeight: 0 },
  art: { maxWidth: 'min(78vw, 20rem)', maxHeight: '100%', aspectRatio: '1 / 1', objectFit: 'cover', borderRadius: 'var(--radius-lg)', boxShadow: '0 1rem 3rem rgba(0,0,0,0.5)' },
  artFallback: {
    width: 'min(78vw, 20rem)', aspectRatio: '1 / 1', display: 'grid', placeItems: 'center',
    fontSize: '5rem', color: 'var(--text-muted)', background: 'var(--bg-elev-2)', borderRadius: 'var(--radius-lg)',
  },

  meta: { textAlign: 'center' },
  title: { fontSize: '1.35rem', fontWeight: 700, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  subtitle: { fontSize: '1rem', color: 'var(--text-muted)', marginTop: '0.25rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },

  scrubWrap: { display: 'flex', flexDirection: 'column', gap: '0.25rem' },
  barHit: { padding: '0.75rem 0', touchAction: 'none', cursor: 'pointer' },
  barTrack: { position: 'relative', height: '0.3rem', background: 'var(--bg-elev-3)', borderRadius: '999px' },
  barFill: { position: 'absolute', top: 0, left: 0, height: '100%', background: 'var(--accent)', borderRadius: '999px' },
  barThumb: { position: 'absolute', top: '50%', width: '1rem', height: '1rem', marginLeft: '-0.5rem', transform: 'translateY(-50%)', borderRadius: '50%', background: 'var(--accent)' },
  timeRow: { display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: 'var(--text-muted)', fontVariantNumeric: 'tabular-nums' },

  transport: { display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '1.25rem' },
  sideBtn: { width: '3rem', height: '3rem', display: 'grid', placeItems: 'center', fontSize: '1.7rem', color: 'var(--text)', background: 'transparent' },
  playBtn: { width: '4.5rem', height: '4.5rem', display: 'grid', placeItems: 'center', fontSize: '2.2rem', color: 'var(--on-accent)', background: 'var(--accent)', borderRadius: '50%' },
}

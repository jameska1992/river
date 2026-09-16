import { Link } from 'react-router-dom'
import { RiFilmLine, RiTv2Line, RiHeadphoneLine, RiMusic2Line, RiImageLine } from 'react-icons/ri'
import { imageUrl } from '../util/imageUrl'
import { progressPct } from '../util/format'

type Kind = 'movie' | 'tvshow' | 'audiobook' | 'album' | 'artist' | 'episode' | 'chapter'

const FALLBACK: Record<Kind, React.ReactNode> = {
  movie: <RiFilmLine />, tvshow: <RiTv2Line />, episode: <RiTv2Line />,
  audiobook: <RiHeadphoneLine />, chapter: <RiHeadphoneLine />,
  album: <RiMusic2Line />, artist: <RiMusic2Line />,
}

export interface PosterCardProps {
  to: string
  title: string
  subtitle?: string
  image?: string
  kind: Kind
  // landscape uses 16:9 (continue-watching / episodes); default is 2:3 poster.
  landscape?: boolean
  // 0–100 completion bar (continue-watching); position + duration.
  position?: number
  duration?: number
}

export function PosterCard({ to, title, subtitle, image, kind, landscape, position, duration }: PosterCardProps) {
  const src = imageUrl(image, landscape ? 'backdrop' : 'poster')
  const pct = position != null && duration != null ? progressPct(position, duration) : 0
  return (
    <Link to={to} style={styles.card}>
      <div style={{ ...styles.thumb, aspectRatio: landscape ? '16 / 9' : '2 / 3' }}>
        {src
          ? <img src={src} alt={title} loading="lazy" style={styles.img} />
          : <div style={styles.fallback}>{FALLBACK[kind] ?? <RiImageLine />}</div>}
        {pct > 0 && (
          <div style={styles.progressTrack}><div style={{ ...styles.progressFill, width: `${pct}%` }} /></div>
        )}
      </div>
      <div style={styles.title}>{title}</div>
      {subtitle && <div style={styles.subtitle}>{subtitle}</div>}
    </Link>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: { display: 'block', color: 'inherit' },
  thumb: {
    position: 'relative',
    width: '100%',
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
  },
  img: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  fallback: {
    width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center',
    color: 'var(--text-muted)', fontSize: '2rem',
  },
  progressTrack: { position: 'absolute', left: 0, right: 0, bottom: 0, height: 3, background: 'rgba(255,255,255,0.25)' },
  progressFill: { height: '100%', background: 'var(--accent)' },
  title: {
    marginTop: '0.4rem', fontSize: '0.85rem', fontWeight: 600,
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  subtitle: {
    fontSize: '0.75rem', color: 'var(--text-muted)',
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
}

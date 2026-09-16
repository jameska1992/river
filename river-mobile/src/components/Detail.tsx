import type { ReactNode } from 'react'
import { RiPlayFill } from 'react-icons/ri'
import { imageUrl } from '../util/imageUrl'

// DetailHero — the shared top of every detail page: backdrop/cover, title,
// a meta line, optional description, and an actions row.
export function DetailHero({
  image, landscape, title, subtitle, meta, description, actions,
}: {
  image?: string
  landscape?: boolean
  title: string
  subtitle?: string
  meta?: string
  description?: string
  actions?: ReactNode
}) {
  const src = imageUrl(image, landscape ? 'backdrop' : 'poster')
  return (
    <div>
      <div style={{ ...styles.art, aspectRatio: landscape ? '16 / 9' : '2 / 3', maxWidth: landscape ? '100%' : '12rem' }}>
        {src && <img src={src} alt={title} style={styles.artImg} />}
      </div>
      <div style={styles.body}>
        <h1 style={styles.title}>{title}</h1>
        {subtitle && <p style={styles.subtitle}>{subtitle}</p>}
        {meta && <p style={styles.meta}>{meta}</p>}
        {actions && <div style={styles.actions}>{actions}</div>}
        {description && <p style={styles.desc}>{description}</p>}
      </div>
    </div>
  )
}

// PlayRow — a tappable list row (episode / track / chapter) with a number,
// title, trailing meta, and a play affordance.
export function PlayRow({
  index, title, meta, onPlay,
}: {
  index?: number | string
  title: string
  meta?: string
  onPlay: () => void
}) {
  return (
    <button onClick={onPlay} style={styles.row}>
      {index != null && <span style={styles.rowIndex}>{index}</span>}
      <span style={styles.rowTitle}>{title}</span>
      {meta && <span style={styles.rowMeta}>{meta}</span>}
      <span style={styles.rowPlay}><RiPlayFill /></span>
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  art: {
    width: '100%', margin: '0 auto', borderRadius: 'var(--radius-md)', overflow: 'hidden',
    background: 'var(--bg-elev-2)',
  },
  artImg: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  body: { padding: '1rem 1.25rem' },
  title: { margin: '0 0 0.25rem', fontSize: '1.5rem', fontWeight: 700 },
  subtitle: { margin: '0 0 0.5rem', color: 'var(--text-muted)' },
  meta: { margin: '0 0 0.75rem', color: 'var(--text-muted)', fontSize: '0.9rem' },
  actions: { display: 'flex', flexWrap: 'wrap', gap: '0.75rem', marginBottom: '1rem' },
  desc: { margin: 0, lineHeight: 1.5, color: 'var(--text)' },
  row: {
    width: '100%', display: 'flex', alignItems: 'center', gap: '0.75rem',
    padding: '0.85rem 1.25rem', textAlign: 'left', borderBottom: '1px solid var(--outline)',
  },
  rowIndex: { width: '1.5rem', color: 'var(--text-muted)', fontVariantNumeric: 'tabular-nums', flexShrink: 0 },
  rowTitle: { flex: 1, minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  rowMeta: { color: 'var(--text-muted)', fontSize: '0.85rem', flexShrink: 0 },
  rowPlay: { color: 'var(--accent)', display: 'flex', flexShrink: 0 },
}

export { RiPlayFill }

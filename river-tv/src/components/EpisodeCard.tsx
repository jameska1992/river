import { useFocusable } from '../hooks/useFocus'
import { useRowEnsureVisible } from './Row'

interface Props {
  number: number
  title: string
  description?: string
  runtime?: number
  available?: boolean
  isSpecial?: boolean
  autoFocus?: boolean
  onSelect?: () => void
}

export function EpisodeCard({
  number, title, description, runtime, available = true, isSpecial = false, autoFocus, onSelect,
}: Props) {
  const ensureRowVisible = useRowEnsureVisible()
  const ref = useFocusable<HTMLDivElement>(available ? onSelect : undefined, {
    autoFocus,
    onFocusChange: focused => { if (focused) ensureRowVisible?.() },
  })

  return (
    <div ref={ref} tabIndex={-1} style={{ ...styles.card, ...(available ? {} : styles.cardDisabled) }}>
      <div style={styles.header}>
        <span style={styles.number}>{isSpecial ? 'SPEC' : `E${number}`}</span>
        {runtime && runtime > 0 && (
          <span style={styles.runtime}>{runtime}m</span>
        )}
      </div>
      <div style={styles.title}>{title || (isSpecial ? 'Special' : `Episode ${number}`)}</div>
      {description && <div style={styles.description}>{description}</div>}
      {!available && <div style={styles.notReady}>Not yet available</div>}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    flex: '0 0 auto',
    width: '22rem',
    minHeight: '12rem',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-md)',
    padding: '1rem 1.25rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    // See PosterCard.
    scrollMargin: '2rem',
  },
  cardDisabled: {
    opacity: 0.55,
  },
  header: {
    display: 'flex',
    alignItems: 'baseline',
    justifyContent: 'space-between',
    gap: '0.5rem',
    color: 'var(--text-muted)',
    fontSize: '0.85rem',
  },
  number: {
    fontWeight: 700,
    fontSize: '0.95rem',
    letterSpacing: '0.02em',
  },
  runtime: {
    fontSize: '0.85rem',
  },
  title: {
    fontSize: '1.05rem',
    fontWeight: 600,
    color: 'var(--text)',
    display: '-webkit-box',
    WebkitLineClamp: 2,
    WebkitBoxOrient: 'vertical',
    overflow: 'hidden',
  },
  description: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
    lineHeight: 1.4,
    display: '-webkit-box',
    WebkitLineClamp: 3,
    WebkitBoxOrient: 'vertical',
    overflow: 'hidden',
  },
  notReady: {
    marginTop: 'auto',
    fontSize: '0.8rem',
    color: '#ffb869',
  },
}

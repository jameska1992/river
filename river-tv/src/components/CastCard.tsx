import { useFocusable } from '../hooks/useFocus'
import { imageUrl } from '../util/imageUrl'
import { useRowEnsureVisible } from './rowContext'

interface Props {
  name: string
  role?: string
  photoUrl?: string
  onSelect?: () => void
  // Page-level scroll hook — fires after the Row's own ensureVisible so
  // pages can override and e.g. scroll the whole page to the bottom.
  onFocus?: () => void
}

export function CastCard({ name, role, photoUrl, onSelect, onFocus }: Props) {
  const ensureRowVisible = useRowEnsureVisible()
  const ref = useFocusable<HTMLDivElement>(onSelect, {
    onFocusChange: focused => {
      if (!focused) return
      ensureRowVisible?.()
      onFocus?.()
    },
  })

  return (
    <div ref={ref} tabIndex={-1} style={styles.card}>
      <div style={styles.photo}>
        {imageUrl(photoUrl) ? (
          <img src={imageUrl(photoUrl)} alt={name} decoding="async" style={styles.img} />
        ) : (
          <div style={styles.placeholder}>{initials(name)}</div>
        )}
      </div>
      <div style={styles.name}>{name}</div>
      {role && <div style={styles.role}>{role}</div>}
    </div>
  )
}

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(w => w[0]!.toUpperCase())
    .join('')
}

const W = '9rem'
const PHOTO_H = '12rem'

const styles: Record<string, React.CSSProperties> = {
  card: {
    flex: '0 0 auto',
    width: W,
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    borderRadius: 'var(--radius-md)',
    // See PosterCard — adds vertical breathing room when scrolled into
    // view from a focus change.
    scrollMargin: '2rem',
  },
  photo: {
    width: W,
    height: PHOTO_H,
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
  },
  img: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  placeholder: {
    width: '100%',
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    fontSize: '2rem',
    fontWeight: 700,
    color: 'var(--text-muted)',
  },
  name: {
    fontSize: '0.95rem',
    fontWeight: 600,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  role: {
    fontSize: '0.8rem',
    color: 'var(--text-muted)',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
}

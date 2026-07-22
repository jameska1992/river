import { useFocusable } from '../hooks/useFocus'
import { imageUrl } from '../util/imageUrl'
import { useRowEnsureVisible } from './rowContext'

interface Props {
  name: string
  itemCount: number
  covers: string[]
  // When true the card spans its grid cell instead of using a fixed
  // pixel width — matches PosterCard's fill mode for browse grids.
  fill?: boolean
  autoFocus?: boolean
  overrides?: { up?: string; down?: string; left?: string; right?: string }
  onSelect?: () => void
  onFocus?: () => void
}

/*
 * Mirrors the web CollectionPreview/Card art rules:
 *   0 covers   → folder placeholder
 *   1–3 covers → single hero image (first cover)
 *   4+ covers  → 2×2 collage of the first four
 */
export function CollectionCard({
  name, itemCount, covers, fill, autoFocus, overrides, onSelect, onFocus,
}: Props) {
  const ensureRowVisible = useRowEnsureVisible()
  const ref = useFocusable<HTMLDivElement>(onSelect, {
    autoFocus,
    overrides,
    onFocusChange: focused => {
      if (!focused) return
      ensureRowVisible?.()
      onFocus?.()
    },
  })

  const cardStyle: React.CSSProperties = fill
    ? { ...styles.card, width: '100%', minWidth: 0 }
    : styles.card
  const artStyle: React.CSSProperties = fill
    ? { ...styles.art, width: '100%', height: 'auto', aspectRatio: '16 / 9' }
    : styles.art

  return (
    <div ref={ref} tabIndex={-1} style={cardStyle}>
      <div style={artStyle}>
        {covers.length === 0 ? (
          <div style={styles.empty}>▦</div>
        ) : covers.length < 4 ? (
          // See PosterCard — lazy-loading sits inside a nested scroll
          // container and can starve the WebView of any image fetches.
          <img src={imageUrl(covers[0])} alt={name} decoding="async" style={styles.single} />
        ) : (
          <div style={styles.collage}>
            {covers.slice(0, 4).map((src, i) => (
              <img key={i} src={imageUrl(src)} alt="" decoding="async" style={styles.collageImg} />
            ))}
          </div>
        )}
      </div>
      <div style={styles.name}>{name}</div>
      <div style={styles.count}>{itemCount} {itemCount === 1 ? 'item' : 'items'}</div>
    </div>
  )
}

const W = '20rem'
const H = '11.25rem' // matches landscape PosterCard for visual rhythm

const styles: Record<string, React.CSSProperties> = {
  card: {
    flex: '0 0 auto',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    width: W,
    borderRadius: 'var(--radius-md)',
    // See PosterCard for the rationale — keeps the focused card off the
    // viewport edge after a row-to-row D-pad jump.
    scrollMargin: '2rem',
  },
  art: {
    width: W,
    height: H,
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: 'var(--bg-elev-2)',
  },
  single: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  collage: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gridTemplateRows: '1fr 1fr',
    width: '100%',
    height: '100%',
    gap: '2px',
    background: 'var(--bg-elev-2)',
  },
  collageImg: {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },
  empty: {
    width: '100%',
    height: '100%',
    display: 'grid',
    placeItems: 'center',
    fontSize: '3rem',
    color: 'var(--text-muted)',
  },
  name: {
    fontSize: '1rem',
    fontWeight: 600,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  count: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
  },
}

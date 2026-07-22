import { useFocusable } from '../hooks/useFocus'
import { imageUrl } from '../util/imageUrl'
import { useRowEnsureVisible } from './Row'

interface Props {
  title: string
  subtitle?: string
  imageSrc?: string
  aspect?: 'portrait' | 'landscape' | 'square'
  // When true, the card fills its container width (used by grid layouts)
  // and the poster sizes via aspect-ratio. When false the card uses a
  // fixed pixel width (used by horizontal scrollers).
  fill?: boolean
  autoFocus?: boolean
  overrides?: { up?: string; down?: string; left?: string; right?: string }
  onSelect?: () => void
  onFocus?: () => void
}

export function PosterCard({
  title, subtitle, imageSrc, aspect = 'portrait', fill, autoFocus, overrides, onSelect, onFocus,
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

  const fixedW =
    aspect === 'portrait'  ? '12rem' :
    aspect === 'square'    ? '14rem' :
                             '20rem'
  const fixedH =
    aspect === 'portrait'  ? '18rem' :
    aspect === 'square'    ? '14rem' :
                             '11.25rem'
  const aspectRatio =
    aspect === 'portrait'  ? '2 / 3' :
    aspect === 'square'    ? '1 / 1' :
                             '16 / 9'

  const cardStyle: React.CSSProperties = fill
    ? { ...styles.card, width: '100%', minWidth: 0 }
    : { ...styles.card, width: fixedW }

  const posterStyle: React.CSSProperties = fill
    ? { ...styles.poster, width: '100%', aspectRatio }
    : { ...styles.poster, width: fixedW, height: fixedH }

  return (
    <div ref={ref} tabIndex={-1} style={cardStyle}>
      <div style={posterStyle}>
        {imageUrl(imageSrc) ? (
          // No loading="lazy" here — these cards live inside nested
          // overflow:auto scrollers, and the native lazy-load heuristic
          // only watches the document viewport. In the Android WebView
          // it sometimes never triggers a fetch.
          <img src={imageUrl(imageSrc)} alt={title} decoding="async" style={styles.img} />
        ) : (
          <div style={styles.placeholder}>{title.charAt(0)}</div>
        )}
      </div>
      <div style={styles.title}>{title}</div>
      {subtitle && <div style={styles.subtitle}>{subtitle}</div>}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    flex: '0 0 auto',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    borderRadius: 'var(--radius-md)',
    // scroll-margin extends the box the browser tries to keep on
    // screen when scrollIntoView() is called on this card. Result: the
    // focused card never sits flush against the viewport edge after a
    // D-pad row jump — there's always ~2rem of breathing room above
    // and below it.
    scrollMargin: '2rem',
  },
  poster: {
    background: 'var(--bg-elev-2)',
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
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
    fontSize: '3rem',
    fontWeight: 700,
    color: 'var(--text-muted)',
  },
  title: {
    fontSize: '1rem',
    fontWeight: 600,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  subtitle: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
  },
}

import { useEffect, useState } from 'react'
import type { RecentlyAddedItem } from '../api'
import { useFocusable } from '../hooks/useFocus'
import { imageUrl } from '../util/imageUrl'

const SLIDE_DURATION = 7000

interface Props {
  items: RecentlyAddedItem[]
  // Invoked when the user activates "More Info" on the currently shown
  // slide. Future-wired to navigate to the movie / show detail page.
  onMoreInfo?: (item: RecentlyAddedItem) => void
}

/*
 * Auto-rotating hero. Non-focusable on purpose so D-pad keeps moving
 * between carousel rows underneath. Height is bounded to ~22 rem so the
 * first carousel sits at least partly inside the visible viewport on
 * 1080p without scrolling.
 */
export function HeroBanner({ items, onMoreInfo }: Props) {
  const [index, setIndex] = useState(0)
  // Pause auto-rotation while the More Info button is focused so the
  // slide can't slip out from under the user as they press OK.
  const [paused, setPaused] = useState(false)

  useEffect(() => {
    if (items.length < 2 || paused) return
    const id = setInterval(() => {
      setIndex(i => (i + 1) % items.length)
    }, SLIDE_DURATION)
    return () => clearInterval(id)
  }, [items.length, paused])

  if (items.length === 0) return null
  const item = items[Math.min(index, items.length - 1)]

  const description = item.description.length > 220
    ? item.description.slice(0, 220).trimEnd() + '…'
    : item.description

  return (
    <div style={styles.hero}>
      {imageUrl(item.backdrop_path, 'backdrop') ? (
        <img
          src={imageUrl(item.backdrop_path, 'backdrop')}
          alt=""
          decoding="async"
          // Re-key per item so React swaps the <img> instead of mutating
          // src — that lets the browser cross-fade via CSS transition on
          // the new node rather than flashing while loading.
          key={item.id}
          style={styles.backdrop}
        />
      ) : (
        <div style={styles.backdropFallback} />
      )}
      <div style={styles.gradientBottom} />
      <div style={styles.gradientLeft} />

      <div style={styles.content}>
        <h1 style={styles.title}>{item.title}</h1>
        <div style={styles.attrs}>
          {item.year > 0 && <span>{item.year}</span>}
          {item.rating > 0 && <span>★ {item.rating.toFixed(1)}</span>}
          <span style={styles.kind}>{item.media_type === 'movie' ? 'Movie' : 'Series'}</span>
        </div>
        {description && <p style={styles.description}>{description}</p>}

        <div style={styles.actions}>
          <MoreInfoButton
            onSelect={() => onMoreInfo?.(item)}
            onFocusChange={setPaused}
          />
        </div>
      </div>

      {items.length > 1 && (
        <div style={styles.dots}>
          {items.map((it, i) => (
            <span
              key={it.id}
              style={{ ...styles.dot, ...(i === index ? styles.dotActive : {}) }}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function MoreInfoButton({
  onSelect,
  onFocusChange,
}: {
  onSelect: () => void
  onFocusChange: (focused: boolean) => void
}) {
  // overrides.left → sidebar, since sidebar items are tagged out of
  // spatial nav. Without this, pressing Left on the hero button would
  // do nothing and the only way to reach the sidebar from the hero
  // would be the Back key.
  // skipInitialFocus — the homepage routes initial focus to the first
  // card of the top carousel, not the hero. Without this, the hero
  // button would claim focus on mount before any card with autoFocus
  // had a chance to register.
  const ref = useFocusable<HTMLButtonElement>(onSelect, {
    overrides: { left: 'sidebar' },
    skipInitialFocus: true,
    onFocusChange,
  })
  return (
    <button ref={ref} tabIndex={-1} onClick={onSelect} style={styles.moreInfoBtn}>
      More Info
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  hero: {
    position: 'relative',
    height: '22rem',
    // flexShrink: 0 — main is a flex column with overflow:auto. Without
    // this the flex algorithm would shrink the hero (and any other
    // fixed-height children) to fit the container's intrinsic height,
    // collapsing the banner to nothing instead of letting it overflow
    // and the container scroll.
    flexShrink: 0,
    margin: '0 var(--safe-x)',
    borderRadius: 'var(--radius-lg)',
    overflow: 'hidden',
    background: 'var(--bg-elev)',
  },
  backdrop: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    objectFit: 'cover',
    animation: 'heroFadeIn 600ms ease-out both',
  },
  backdropFallback: {
    position: 'absolute',
    inset: 0,
    background: 'linear-gradient(135deg, #2a2a2a 0%, #131313 100%)',
  },
  gradientBottom: {
    position: 'absolute',
    inset: 0,
    background: 'linear-gradient(to top, rgba(19,19,19,0.95) 0%, rgba(19,19,19,0.2) 50%, transparent 100%)',
  },
  gradientLeft: {
    position: 'absolute',
    inset: 0,
    background: 'linear-gradient(to right, rgba(19,19,19,0.85) 0%, rgba(19,19,19,0) 50%)',
  },
  content: {
    position: 'absolute',
    left: '2.5rem',
    right: '2.5rem',
    bottom: '2rem',
    maxWidth: '40rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
  },
  title: {
    margin: 0,
    fontSize: '2.5rem',
    fontWeight: 700,
    letterSpacing: '-0.02em',
    lineHeight: 1.1,
    textShadow: '0 0.15rem 0.5rem rgba(0,0,0,0.5)',
  },
  attrs: {
    display: 'flex',
    gap: '1rem',
    color: 'var(--text-muted)',
    fontSize: '1rem',
  },
  kind: {
    background: 'rgba(255,255,255,0.12)',
    color: 'var(--text)',
    padding: '0.125rem 0.625rem',
    borderRadius: '999px',
    fontSize: '0.85rem',
    fontWeight: 600,
  },
  description: {
    margin: 0,
    fontSize: '0.95rem',
    color: 'var(--text-muted)',
    lineHeight: 1.45,
    textShadow: '0 0.1rem 0.3rem rgba(0,0,0,0.5)',
  },
  actions: {
    display: 'flex',
    gap: '0.75rem',
    marginTop: '0.75rem',
  },
  moreInfoBtn: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.75rem 1.5rem',
    fontSize: '1rem',
    fontWeight: 600,
    color: 'var(--text)',
    background: 'rgba(255,255,255,0.16)',
    borderRadius: 'var(--radius-md)',
    backdropFilter: 'blur(8px)',
  },
  dots: {
    position: 'absolute',
    right: '1.5rem',
    bottom: '1rem',
    display: 'flex',
    gap: '0.4rem',
  },
  dot: {
    width: '0.5rem',
    height: '0.5rem',
    borderRadius: '50%',
    background: 'rgba(255,255,255,0.3)',
    transition: 'background 200ms ease, width 200ms ease',
  },
  dotActive: {
    background: 'var(--accent)',
    width: '1.5rem',
    borderRadius: '0.25rem',
  },
}

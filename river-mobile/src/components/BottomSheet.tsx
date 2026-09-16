import { useEffect, type ReactNode } from 'react'
import { RiCheckLine } from 'react-icons/ri'

// A bottom-anchored modal sheet for touch pickers (subtitle / audio track).
// Tapping the scrim or pressing Escape dismisses it. The list scrolls
// independently with overscroll containment and momentum scrolling so a long
// track list can't rubber-band the player behind it — the phone equivalent of
// the #121 subtitle-picker scroll fix.
export function BottomSheet({
  title, onClose, children,
}: {
  title: string
  onClose: () => void
  children: ReactNode
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div style={styles.scrim} onClick={onClose}>
      <div
        style={styles.sheet}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={e => e.stopPropagation()}
      >
        <div style={styles.grabber} />
        <h3 style={styles.title}>{title}</h3>
        <div style={styles.list}>{children}</div>
      </div>
    </div>
  )
}

// One selectable option in a BottomSheet, with a check on the active row.
export function SheetOption({
  label, active, onSelect,
}: {
  label: string
  active: boolean
  onSelect: () => void
}) {
  return (
    <button
      onClick={onSelect}
      aria-pressed={active}
      style={{ ...styles.option, ...(active ? styles.optionActive : {}) }}
    >
      <span style={styles.optionLabel}>{label}</span>
      {active && <RiCheckLine style={styles.check} />}
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  scrim: {
    position: 'fixed', inset: 0, zIndex: 30,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex', alignItems: 'flex-end',
  },
  sheet: {
    width: '100%',
    maxHeight: '70vh',
    display: 'flex', flexDirection: 'column',
    background: 'var(--bg-elev)',
    borderTopLeftRadius: 'var(--radius-lg)',
    borderTopRightRadius: 'var(--radius-lg)',
    padding: '0.5rem 1rem calc(1rem + env(safe-area-inset-bottom, 0px))',
  },
  grabber: {
    width: '2.5rem', height: '0.25rem',
    background: 'var(--outline)', borderRadius: '999px',
    margin: '0.5rem auto 0.75rem',
  },
  title: {
    margin: '0 0 0.5rem', padding: '0 0.25rem',
    fontSize: '0.85rem', fontWeight: 700,
    textTransform: 'uppercase', letterSpacing: '0.05em',
    color: 'var(--text-muted)',
  },
  list: {
    display: 'flex', flexDirection: 'column',
    overflowY: 'auto',
    // Contain the scroll so flinging past the ends doesn't move the player.
    overscrollBehavior: 'contain',
    WebkitOverflowScrolling: 'touch',
  },
  option: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    gap: '1rem',
    padding: '0.9rem 0.75rem',
    minHeight: '3rem',
    fontSize: '1rem', color: 'var(--text)',
    background: 'transparent', textAlign: 'left',
    borderRadius: 'var(--radius-md)',
  },
  optionActive: { color: 'var(--accent)', fontWeight: 600 },
  optionLabel: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  check: { flex: '0 0 auto', fontSize: '1.25rem' },
}

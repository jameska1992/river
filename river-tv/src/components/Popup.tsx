import { useEffect, type ReactNode } from 'react'
import { FocusProvider, useFocusContext } from '../hooks/useFocus'

interface Props {
  onClose: () => void
  children: ReactNode
}

/*
 * Modal overlay that runs its own FocusProvider scope. While open, the
 * parent scope is paused so arrow keys only move focus inside the popup.
 * Click on the backdrop or press Back/Escape to close.
 */
export function Popup({ onClose, children }: Props) {
  const parent = useFocusContext()

  useEffect(() => {
    parent?.setPaused(true)
    return () => { parent?.setPaused(false) }
  }, [parent])

  return (
    <div style={styles.backdrop} onClick={onClose}>
      <FocusProvider onBack={onClose}>
        <div style={styles.panel} onClick={e => e.stopPropagation()}>
          {children}
        </div>
      </FocusProvider>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  backdrop: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0, 0, 0, 0.65)',
    backdropFilter: 'blur(8px)',
    display: 'grid',
    placeItems: 'center',
    zIndex: 100,
  },
  panel: {
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-lg)',
    padding: '1.5rem',
    minWidth: '28rem',
    maxWidth: '90vw',
    maxHeight: '80vh',
    overflowY: 'auto',
    scrollbarWidth: 'none',
    boxShadow: '0 1.5rem 4rem rgba(0,0,0,0.5)',
  },
}

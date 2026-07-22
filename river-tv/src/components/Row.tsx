import { createContext, useCallback, useContext, useMemo, useRef, type ReactNode } from 'react'

interface RowCtx {
  ensureVisible: () => void
}

const Ctx = createContext<RowCtx | null>(null)

/** Cards inside a Row call this to scroll the entire section (title +
 *  cards + a bit of margin) into view when they gain focus. */
export function useRowEnsureVisible() {
  return useContext(Ctx)?.ensureVisible
}

interface Props {
  title: string
  children: ReactNode
}

export function Row({ title, children }: Props) {
  const sectionRef = useRef<HTMLElement>(null)

  // scrollIntoView({block: 'nearest'}) snaps the section fully into the
  // scroll viewport when possible — extended by scroll-margin so the
  // title and a little breathing room sit above/below the cards.
  const ensureVisible = useCallback(() => {
    sectionRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [])

  const ctx = useMemo(() => ({ ensureVisible }), [ensureVisible])

  return (
    <Ctx.Provider value={ctx}>
      <section ref={sectionRef} style={styles.section}>
        <h2 style={styles.title}>{title}</h2>
        <div style={styles.scroller}>
          {children}
        </div>
      </section>
    </Ctx.Provider>
  )
}

const styles: Record<string, React.CSSProperties> = {
  section: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
    // Don't let the parent flex column shrink rows below their
    // intrinsic height when the home page overflows.
    flexShrink: 0,
    // scroll-margin extends the "make visible" target box so the title
    // never sits flush against the top edge and the card bottoms get a
    // little breathing room.
    scrollMarginTop: '1.5rem',
    scrollMarginBottom: '1.5rem',
  },
  title: {
    margin: 0,
    fontSize: '1.4rem',
    fontWeight: 600,
    paddingLeft: 'var(--safe-x)',
  },
  scroller: {
    display: 'flex',
    gap: '1.25rem',
    overflowX: 'auto',
    padding: '0.75rem var(--safe-x)',
    scrollbarWidth: 'none',
  },
}

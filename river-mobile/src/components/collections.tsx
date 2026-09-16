import type { ReactNode } from 'react'

// Row — a horizontal, touch-scrollable rail with a heading. Used for the
// home-screen carousels. Children are fixed-width cards.
export function Row({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section style={rowStyles.section}>
      <h2 style={rowStyles.heading}>{title}</h2>
      <div style={rowStyles.track}>{children}</div>
    </section>
  )
}

const rowStyles: Record<string, React.CSSProperties> = {
  section: { marginBottom: '1.5rem' },
  heading: { margin: '0 0 0.6rem', fontSize: '1.1rem', fontWeight: 700, padding: '0 1.25rem' },
  track: {
    display: 'flex',
    gap: '0.75rem',
    overflowX: 'auto',
    padding: '0 1.25rem 0.25rem',
    scrollbarWidth: 'none',
    WebkitOverflowScrolling: 'touch',
  },
}

// Grid — a responsive grid of poster cards for library/search/watchlist.
export function Grid({ children }: { children: ReactNode }) {
  return <div style={gridStyles.grid}>{children}</div>
}

const gridStyles: Record<string, React.CSSProperties> = {
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(6.5rem, 1fr))',
    gap: '1rem 0.75rem',
    padding: '0 1.25rem',
  },
}

// Fixed-width wrapper so cards in a Row don't collapse in the flex track.
export function RailItem({ width = '8rem', children }: { width?: string; children: ReactNode }) {
  return <div style={{ flex: `0 0 ${width}`, width }}>{children}</div>
}

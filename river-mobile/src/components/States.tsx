// Shared async-state UI: loading, error (with retry), and empty.

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return <div style={styles.center}>{label}</div>
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div style={styles.center}>
      <p style={{ color: 'var(--error)', margin: 0 }}>{message}</p>
      {onRetry && <button className="btn" onClick={onRetry} style={{ marginTop: '1rem' }}>Retry</button>}
    </div>
  )
}

export function EmptyState({ message }: { message: string }) {
  return <div style={{ ...styles.center, color: 'var(--text-muted)' }}>{message}</div>
}

const styles: Record<string, React.CSSProperties> = {
  center: {
    display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    textAlign: 'center', padding: '3rem 1.5rem', color: 'var(--text-muted)',
  },
}

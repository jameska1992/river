import { useAuth } from '../context/authContext'
import { api } from '../api'
import { screen, heading } from './styles'

export default function SettingsPage() {
  const { user, logout } = useAuth()
  return (
    <div style={screen}>
      <h1 style={heading}>Settings</h1>

      <div style={styles.row}>
        <span style={styles.key}>Signed in as</span>
        <span style={styles.val}>{user?.username ?? '—'}</span>
      </div>
      <div style={styles.row}>
        <span style={styles.key}>Server</span>
        <span style={styles.val}>{api.apiBaseURL}</span>
      </div>

      <button className="btn" onClick={() => void logout()} style={styles.signOut}>
        Sign out
      </button>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  row: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: '1rem',
    padding: '0.9rem 0',
    borderBottom: '1px solid var(--outline)',
  },
  key: { color: 'var(--text-muted)' },
  val: { fontWeight: 600, textAlign: 'right', wordBreak: 'break-all' },
  signOut: { marginTop: '1.5rem', color: 'var(--error)' },
}

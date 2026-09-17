import { Link } from 'react-router-dom'
import { RiAddCircleLine, RiArrowRightSLine } from 'react-icons/ri'
import { useAuth } from '../context/authContext'
import { api } from '../api'
import { screen, heading } from './styles'

export default function SettingsPage() {
  const { user, logout } = useAuth()
  return (
    <div style={screen}>
      <h1 style={heading}>Settings</h1>

      <Link to="/request" style={styles.linkRow}>
        <RiAddCircleLine style={{ flex: '0 0 auto' }} />
        <span style={{ flex: 1 }}>Request movies &amp; TV</span>
        <RiArrowRightSLine style={{ flex: '0 0 auto', color: 'var(--text-muted)' }} />
      </Link>

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
  linkRow: {
    display: 'flex', alignItems: 'center', gap: '0.75rem',
    padding: '0.9rem 0', borderBottom: '1px solid var(--outline)',
    color: 'var(--text)', fontWeight: 600,
  },
  key: { color: 'var(--text-muted)' },
  val: { fontWeight: 600, textAlign: 'right', wordBreak: 'break-all' },
  signOut: { marginTop: '1.5rem', color: 'var(--error)' },
}

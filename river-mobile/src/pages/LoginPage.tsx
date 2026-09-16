import { useState, type FormEvent } from 'react'
import { useAuth } from '../context/authContext'
import { api, ApiError } from '../api'

// Touch login. Mirrors river-tv's flow (server URL + credentials) but with
// plain form inputs — no D-pad focus manager. Server picker / remembered
// servers can be layered on later.
export default function LoginPage() {
  const { login } = useAuth()
  const [server, setServer] = useState(api.apiBaseURL)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    api.setBaseURL(server)
    try {
      await login(username, password)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Login failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={styles.wrap}>
      <form onSubmit={onSubmit} style={styles.card}>
        <h1 style={styles.title}>River</h1>
        <p style={styles.subtitle}>Sign in to continue</p>

        <label style={styles.field}>
          <span style={styles.label}>Server</span>
          <input
            className="input"
            value={server}
            onChange={e => setServer(e.target.value)}
            inputMode="url"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            placeholder="http://192.168.1.10:8080/api"
          />
        </label>

        <label style={styles.field}>
          <span style={styles.label}>Username</span>
          <input
            className="input"
            value={username}
            onChange={e => setUsername(e.target.value)}
            autoCapitalize="none"
            autoCorrect="off"
            autoComplete="username"
          />
        </label>

        <label style={styles.field}>
          <span style={styles.label}>Password</span>
          <input
            className="input"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            autoComplete="current-password"
          />
        </label>

        {error && <div style={styles.error}>{error}</div>}

        <button type="submit" className="btn btn-primary" disabled={busy} style={styles.submit}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    minHeight: '100vh',
    display: 'grid',
    placeItems: 'center',
    padding: '1.5rem',
    background: 'radial-gradient(1200px 800px at 30% 20%, #2a2a2a 0%, #131313 60%)',
  },
  card: {
    width: '100%',
    maxWidth: '26rem',
    padding: '2rem 1.5rem',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-lg)',
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
  },
  title: { fontFamily: 'var(--font-logo)', fontSize: '2.5rem', margin: 0, fontWeight: 700 },
  subtitle: { margin: '0 0 0.5rem', color: 'var(--text-muted)' },
  field: { display: 'flex', flexDirection: 'column', gap: '0.4rem' },
  label: { fontSize: '0.85rem', color: 'var(--text-muted)' },
  submit: { marginTop: '0.5rem', width: '100%', padding: '1rem' },
  error: {
    background: 'rgba(255, 80, 80, 0.12)',
    color: 'var(--error)',
    padding: '0.75rem 1rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '0.95rem',
  },
}

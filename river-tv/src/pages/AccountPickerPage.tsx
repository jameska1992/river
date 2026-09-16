import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/authContext'
import type { SavedAccount } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'

// AccountPickerPage — the launch screen when this TV remembers ≥1 account,
// and the in-app "Change account" screen. Pick a tile to sign in with no
// password (cached refresh token), add another account, or (in Edit mode)
// remove one. Fully D-pad operable.
export default function AccountPickerPage() {
  const { user, accounts, switchAccount, removeAccount } = useAuth()
  const navigate = useNavigate()
  const [busy, setBusy] = useState<string | null>(null)
  const [editing, setEditing] = useState(false)

  // When opened from inside the app (Change account, so a session is still
  // active), Back returns to where they were rather than trapping them here.
  useEffect(() => {
    if (!user) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' || e.key === 'GoBack' || e.key === 'Backspace') {
        e.preventDefault()
        navigate(-1)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [user, navigate])

  const pick = async (acc: SavedAccount) => {
    setBusy(acc.id)
    try {
      await switchAccount(acc.id)
      navigate('/')
    } catch {
      // Stale/revoked cached token — keep the account, prompt for a password.
      navigate('/login', { state: { username: acc.username } })
    } finally {
      setBusy(null)
    }
  }

  const remove = async (acc: SavedAccount) => {
    await removeAccount(acc.id)
    // If that emptied the list, fall through to the login form.
    if (accounts.length <= 1) navigate('/login')
  }

  return (
    <FocusProvider>
      <div style={styles.wrap}>
        <h1 style={styles.title}>Who’s watching?</h1>

        <div style={styles.tiles}>
          {accounts.map((acc, i) => (
            <AccountTile
              key={acc.id}
              account={acc}
              autoFocus={i === 0}
              busy={busy === acc.id}
              editing={editing}
              onPick={() => void pick(acc)}
              onRemove={() => void remove(acc)}
            />
          ))}
          <AddTile onSelect={() => navigate('/login')} />
        </div>

        <EditToggle editing={editing} onToggle={() => setEditing(e => !e)} />
      </div>
    </FocusProvider>
  )
}

function initials(name: string): string {
  const parts = name.trim().split(/[\s._-]+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function AccountTile({
  account, autoFocus, busy, editing, onPick, onRemove,
}: {
  account: SavedAccount
  autoFocus: boolean
  busy: boolean
  editing: boolean
  onPick: () => void
  onRemove: () => void
}) {
  const [focused, setFocused] = useState(false)
  const ref = useFocusable<HTMLButtonElement>(
    editing ? onRemove : onPick,
    { autoFocus, onFocusChange: setFocused },
  )
  return (
    <button
      ref={ref}
      tabIndex={-1}
      type="button"
      onClick={editing ? onRemove : onPick}
      style={{ ...styles.tile, ...(focused ? styles.tileFocused : {}) }}
    >
      <span style={styles.avatar}>{busy ? '…' : initials(account.username)}</span>
      <span style={styles.tileName}>{account.username}</span>
      {editing && <span style={styles.removeBadge}>Remove ✕</span>}
    </button>
  )
}

function AddTile({ onSelect }: { onSelect: () => void }) {
  const [focused, setFocused] = useState(false)
  const ref = useFocusable<HTMLButtonElement>(onSelect, { onFocusChange: setFocused })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      type="button"
      onClick={onSelect}
      style={{ ...styles.tile, ...(focused ? styles.tileFocused : {}) }}
    >
      <span style={{ ...styles.avatar, ...styles.avatarAdd }}>＋</span>
      <span style={styles.tileName}>Add account</span>
    </button>
  )
}

function EditToggle({ editing, onToggle }: { editing: boolean; onToggle: () => void }) {
  const [focused, setFocused] = useState(false)
  const ref = useFocusable<HTMLButtonElement>(onToggle, { onFocusChange: setFocused })
  return (
    <button
      ref={ref}
      tabIndex={-1}
      type="button"
      onClick={onToggle}
      style={{ ...styles.editBtn, ...(focused ? styles.editBtnFocused : {}) }}
    >
      {editing ? 'Done' : 'Edit accounts'}
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '2.5rem',
    background: 'radial-gradient(1200px 800px at 30% 20%, #2a2a2a 0%, #131313 60%)',
  },
  title: {
    fontFamily: 'var(--font-logo)',
    fontSize: '2.75rem',
    margin: 0,
    fontWeight: 700,
  },
  tiles: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '2rem',
    justifyContent: 'center',
    maxWidth: '60rem',
  },
  tile: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '0.9rem',
    width: '11rem',
    padding: '1.5rem 1rem',
    background: 'transparent',
    borderRadius: 'var(--radius-lg)',
    transition: 'transform 0.12s ease, background 0.12s ease',
  },
  tileFocused: {
    background: 'var(--bg-elev-2)',
    transform: 'scale(1.06)',
    boxShadow: '0 0 0 3px var(--accent)',
  },
  avatar: {
    width: '7rem',
    height: '7rem',
    borderRadius: '50%',
    display: 'grid',
    placeItems: 'center',
    fontSize: '2.5rem',
    fontWeight: 700,
    color: 'var(--on-accent)',
    background: 'linear-gradient(135deg, var(--accent) 0%, #6a5acd 100%)',
  },
  avatarAdd: {
    background: 'var(--bg-elev-2)',
    color: 'var(--text-muted)',
    border: '2px dashed rgba(255,255,255,0.2)',
  },
  tileName: {
    fontSize: '1.15rem',
    fontWeight: 600,
    maxWidth: '10rem',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  removeBadge: {
    fontSize: '0.85rem',
    color: 'var(--error)',
    fontWeight: 600,
  },
  editBtn: {
    background: 'transparent',
    color: 'var(--text-muted)',
    padding: '0.6rem 1.4rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1rem',
    fontWeight: 600,
  },
  editBtnFocused: {
    background: 'var(--bg-elev-2)',
    color: 'var(--text)',
    boxShadow: '0 0 0 2px var(--accent)',
  },
}

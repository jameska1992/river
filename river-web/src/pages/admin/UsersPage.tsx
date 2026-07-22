import { type FormEvent, useEffect, useRef, useState } from 'react'
import {
  RiAddLine, RiCloseLine, RiDeleteBinLine, RiEditLine,
  RiFilmLine, RiHistoryLine, RiKeyLine, RiShieldLine, RiTv2Line, RiUserLine,
} from 'react-icons/ri'
import { useAuth } from '../../context/AuthContext'
import { api, ApiError } from '../../api'
import type { ActivityItem, User } from '../../api'
import styles from './UsersPage.module.css'

// ── helpers ───────────────────────────────────────────────

function relativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function formatDuration(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
  return `${m}:${String(sec).padStart(2, '0')}`
}

// ── page ─────────────────────────────────────────────────

export function UsersPage() {
  const { user: me } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<User | null>(null)
  const [pwdTarget, setPwdTarget] = useState<User | null>(null)
  const [activityTarget, setActivityTarget] = useState<User | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)

  const reload = () => {
    setLoading(true)
    api.listUsers()
      .then(setUsers)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load users'))
      .finally(() => setLoading(false))
  }

  // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount; reload() sets state from the API response
  useEffect(() => { reload() }, [])

  return (
    <div>
      <div className={styles.header}>
        <h1 className="headline-sm">Users</h1>
        <button className="btn btn-primary" onClick={() => setCreateOpen(true)}>
          <RiAddLine size={16} />
          Add User
        </button>
      </div>

      {error && <p className={styles.pageError}>{error}</p>}

      {loading ? (
        <div className={styles.skeletonList}>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className={`${styles.skeletonRow} skeleton`} />
          ))}
        </div>
      ) : (
        <div className={styles.table}>
          <div className={styles.tableHead}>
            <span>User</span>
            <span>Role</span>
            <span>Joined</span>
            <span />
          </div>
          {users.map(u => (
            <div key={u.id} className={styles.tableRow}>
              <div className={styles.userCell}>
                <div className={styles.avatar}>{u.username[0].toUpperCase()}</div>
                <div>
                  <p className={`label-md ${styles.username}`}>{u.username}</p>
                  <p className={`label-sm ${styles.email}`}>{u.email}</p>
                </div>
              </div>
              <div>
                <span className={`badge ${u.role === 'admin' ? 'badge-primary' : ''} ${styles.roleBadge}`}>
                  {u.role === 'admin' ? <RiShieldLine size={11} /> : <RiUserLine size={11} />}
                  {u.role}
                </span>
              </div>
              <span className="label-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
                {relativeTime(u.created_at)}
              </span>
              <div className={styles.actions}>
                <button className="btn btn-icon" title="Edit" onClick={() => setEditTarget(u)}>
                  <RiEditLine size={16} />
                </button>
                <button className="btn btn-icon" title="Set password" onClick={() => setPwdTarget(u)}>
                  <RiKeyLine size={16} />
                </button>
                <button className="btn btn-icon" title="Activity" onClick={() => setActivityTarget(u)}>
                  <RiHistoryLine size={16} />
                </button>
                <button
                  className={`btn btn-icon ${styles.deleteBtn}`}
                  title="Delete"
                  disabled={u.id === me?.id}
                  onClick={() => setDeleteTarget(u)}
                >
                  <RiDeleteBinLine size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {createOpen && (
        <CreateUserModal
          onSave={u => { setUsers(prev => [...prev, u]); setCreateOpen(false) }}
          onClose={() => setCreateOpen(false)}
        />
      )}
      {editTarget && (
        <EditUserModal
          user={editTarget}
          onSave={u => { setUsers(prev => prev.map(x => x.id === u.id ? u : x)); setEditTarget(null) }}
          onClose={() => setEditTarget(null)}
        />
      )}
      {pwdTarget && (
        <SetPasswordModal user={pwdTarget} onClose={() => setPwdTarget(null)} />
      )}
      {activityTarget && (
        <ActivityModal user={activityTarget} onClose={() => setActivityTarget(null)} />
      )}
      {deleteTarget && (
        <DeleteModal
          user={deleteTarget}
          onDelete={() => { setUsers(prev => prev.filter(x => x.id !== deleteTarget.id)); setDeleteTarget(null) }}
          onClose={() => setDeleteTarget(null)}
        />
      )}
    </div>
  )
}

// ── modal shell ───────────────────────────────────────────

function Modal({ title, onClose, children, wide }: {
  title: string; onClose: () => void; children: React.ReactNode; wide?: boolean
}) {
  const overlayRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])
  return (
    <div
      className={styles.overlay}
      ref={overlayRef}
      onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className={`card ${styles.dialog} ${wide ? styles.dialogWide : ''}`}>
        <div className={styles.dialogHeader}>
          <h2 className="headline-sm">{title}</h2>
          <button className="btn btn-icon" onClick={onClose}><RiCloseLine size={20} /></button>
        </div>
        {children}
      </div>
    </div>
  )
}

// ── create user ───────────────────────────────────────────

function CreateUserModal({ onSave, onClose }: { onSave: (u: User) => void; onClose: () => void }) {
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'user' | 'admin'>('user')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setError('')
    try {
      const u = await api.createUser({ username, email, password, role })
      onSave(u)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create user')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title="Add User" onClose={onClose}>
      <form className={styles.form} onSubmit={handleSubmit}>
        <UserFields
          username={username} setUsername={setUsername}
          email={email} setEmail={setEmail}
          role={role} setRole={setRole}
          password={password} setPassword={setPassword}
          showPassword
        />
        {error && <p className={styles.formError}>{error}</p>}
        <div className={styles.dialogFooter}>
          <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Creating…' : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

// ── edit user ─────────────────────────────────────────────

function EditUserModal({ user, onSave, onClose }: { user: User; onSave: (u: User) => void; onClose: () => void }) {
  const [username, setUsername] = useState(user.username)
  const [email, setEmail] = useState(user.email)
  const [role, setRole] = useState<'user' | 'admin'>(user.role)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setError('')
    try {
      const u = await api.updateUser(user.id, { username, email, role })
      onSave(u)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to update user')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title="Edit User" onClose={onClose}>
      <form className={styles.form} onSubmit={handleSubmit}>
        <UserFields
          username={username} setUsername={setUsername}
          email={email} setEmail={setEmail}
          role={role} setRole={setRole}
        />
        {error && <p className={styles.formError}>{error}</p>}
        <div className={styles.dialogFooter}>
          <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

// ── shared user fields ────────────────────────────────────

function UserFields({ username, setUsername, email, setEmail, role, setRole, password, setPassword, showPassword }: {
  username: string; setUsername: (v: string) => void
  email: string; setEmail: (v: string) => void
  role: 'user' | 'admin'; setRole: (v: 'user' | 'admin') => void
  password?: string; setPassword?: (v: string) => void
  showPassword?: boolean
}) {
  return (
    <>
      <label className={styles.field}>
        <span className="label-sm">Username</span>
        <input className="input" value={username} onChange={e => setUsername(e.target.value)} required minLength={3} maxLength={32} />
      </label>
      <label className={styles.field}>
        <span className="label-sm">Email</span>
        <input className="input" type="email" value={email} onChange={e => setEmail(e.target.value)} required />
      </label>
      {showPassword && (
        <label className={styles.field}>
          <span className="label-sm">Password</span>
          <input className="input" type="password" value={password ?? ''} onChange={e => setPassword?.(e.target.value)} required minLength={8} />
        </label>
      )}
      <div className={styles.field}>
        <span className="label-sm">Role</span>
        <div className={styles.roleRow}>
          {(['user', 'admin'] as const).map(r => (
            <button
              key={r}
              type="button"
              className={`${styles.roleOption} ${role === r ? styles.roleOptionActive : ''}`}
              onClick={() => setRole(r)}
            >
              {r === 'admin' ? <RiShieldLine size={15} /> : <RiUserLine size={15} />}
              {r}
            </button>
          ))}
        </div>
      </div>
    </>
  )
}

// ── set password ──────────────────────────────────────────

function SetPasswordModal({ user, onClose }: { user: User; onClose: () => void }) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (password !== confirm) { setError('Passwords do not match'); return }
    setSaving(true); setError('')
    try {
      await api.setUserPassword(user.id, password)
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to set password')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={`Set Password — ${user.username}`} onClose={onClose}>
      {done ? (
        <div className={styles.successMsg}>
          <p className="body-md">Password updated successfully.</p>
          <div className={styles.dialogFooter}>
            <button className="btn btn-primary" onClick={onClose}>Done</button>
          </div>
        </div>
      ) : (
        <form className={styles.form} onSubmit={handleSubmit}>
          <label className={styles.field}>
            <span className="label-sm">New password</span>
            <input className="input" type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} autoFocus />
          </label>
          <label className={styles.field}>
            <span className="label-sm">Confirm password</span>
            <input className="input" type="password" value={confirm} onChange={e => setConfirm(e.target.value)} required minLength={8} />
          </label>
          {error && <p className={styles.formError}>{error}</p>}
          <div className={styles.dialogFooter}>
            <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Set Password'}
            </button>
          </div>
        </form>
      )}
    </Modal>
  )
}

// ── activity ──────────────────────────────────────────────

function ActivityModal({ user, onClose }: { user: User; onClose: () => void }) {
  const [items, setItems] = useState<ActivityItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.getUserActivity(user.id)
      .then(setItems)
      .finally(() => setLoading(false))
  }, [user.id])

  return (
    <Modal title={`Activity — ${user.username}`} onClose={onClose} wide>
      {loading ? (
        <div className={styles.activityList}>
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className={`${styles.activitySkeleton} skeleton`} />
          ))}
        </div>
      ) : items.length === 0 ? (
        <p className={`body-md ${styles.empty}`}>No activity yet.</p>
      ) : (
        <div className={styles.activityList}>
          {items.map((item, i) => (
            <ActivityRow key={i} item={item} />
          ))}
        </div>
      )}
    </Modal>
  )
}

function ActivityRow({ item }: { item: ActivityItem }) {
  const ratio = item.duration > 0 ? Math.min(item.position / item.duration, 1) : 0
  const icon = item.media_type === 'movie' ? <RiFilmLine size={14} /> : <RiTv2Line size={14} />
  return (
    <div className={styles.activityRow}>
      <div className={styles.activityIcon}>{icon}</div>
      <div className={styles.activityInfo}>
        <p className="label-md">
          {item.show_title ? `${item.show_title} · ` : ''}{item.title}
          {item.completed && <span className={`badge badge-primary ${styles.completedBadge}`}>Completed</span>}
        </p>
        <div className={styles.progressBar}>
          <div className={styles.progressFill} style={{ width: `${ratio * 100}%` }} />
        </div>
        <p className="label-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
          {formatDuration(item.position)} / {formatDuration(item.duration)} · {relativeTime(item.updated_at)}
        </p>
      </div>
    </div>
  )
}

// ── delete confirm ────────────────────────────────────────

function DeleteModal({ user, onDelete, onClose }: { user: User; onDelete: () => void; onClose: () => void }) {
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState('')

  async function handleDelete() {
    setDeleting(true); setError('')
    try {
      await api.deleteUser(user.id)
      onDelete()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to delete user')
      setDeleting(false)
    }
  }

  return (
    <Modal title="Delete User" onClose={onClose}>
      <div className={styles.form}>
        <p className="body-md">
          Delete <strong>{user.username}</strong>? This cannot be undone.
        </p>
        {error && <p className={styles.formError}>{error}</p>}
        <div className={styles.dialogFooter}>
          <button className="btn" onClick={onClose} disabled={deleting}>Cancel</button>
          <button className={`btn ${styles.btnDanger}`} onClick={handleDelete} disabled={deleting}>
            {deleting ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </Modal>
  )
}

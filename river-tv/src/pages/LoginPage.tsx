import { useRef, useState, type ChangeEvent, type FormEvent } from 'react'
import { useAuth } from '../context/authContext'
import { api, ApiError } from '../api'
import { FocusProvider, useFocusable } from '../hooks/useFocus'
import { Popup } from '../components/Popup'

export default function LoginPage() {
  const { login } = useAuth()
  const [server, setServer] = useState(api.apiBaseURL)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)

  async function onSubmit(e?: FormEvent) {
    e?.preventDefault()
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
    <FocusProvider>
      <div style={styles.wrap}>
        <form onSubmit={onSubmit} style={styles.card}>
          <h1 style={styles.title}>River</h1>
          <p style={styles.subtitle}>Sign in to continue</p>

          <ServerButton url={server} onSelect={() => setPickerOpen(true)} />

          <FocusableInput
            label="Username"
            value={username}
            onChange={setUsername}
            autoFocus
            autoComplete="username"
          />

          <FocusableInput
            label="Password"
            value={password}
            onChange={setPassword}
            type="password"
            autoComplete="current-password"
          />

          {error && <div style={styles.error}>{error}</div>}

          <SubmitButton busy={busy} onSelect={() => void onSubmit()} />
        </form>

        {pickerOpen && (
          <ServerPicker
            current={server}
            onClose={() => setPickerOpen(false)}
            onChoose={url => { setServer(url); setPickerOpen(false) }}
          />
        )}
      </div>
    </FocusProvider>
  )
}

// ── Login form widgets ─────────────────────────────────────

function ServerButton({ url, onSelect }: { url: string; onSelect: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect)
  return (
    <button ref={ref} tabIndex={-1} type="button" onClick={onSelect} style={styles.serverBtn}>
      <span style={styles.serverBtnLabel}>Server</span>
      <span style={styles.serverBtnValue}>{url || 'Choose…'}</span>
      <span style={styles.serverBtnChevron}>›</span>
    </button>
  )
}

function FocusableInput({
  label, value, onChange, type, autoFocus, autoComplete,
}: {
  label: string
  value: string
  onChange: (next: string) => void
  type?: string
  autoFocus?: boolean
  autoComplete?: string
}) {
  // Wrapper takes focus from the spatial manager; the underlying
  // <input> only receives DOM focus (and therefore only opens the
  // WebView's system keyboard) when the user explicitly presses OK
  // on the focused row. That keeps arrow-key traversal silent so
  // landing on a field while scrolling past doesn't pop the keyboard.
  // Blur on Escape so the user can D-pad-back out of the field once
  // they're done typing.
  const inputRef = useRef<HTMLInputElement>(null)
  const wrapRef = useFocusable<HTMLLabelElement>(
    () => inputRef.current?.focus(),
    { autoFocus },
  )
  return (
    <label ref={wrapRef} tabIndex={-1} style={styles.label}>
      <span style={styles.labelText}>{label}</span>
      <input
        ref={inputRef}
        type={type ?? 'text'}
        value={value}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onKeyDown={e => {
          // Esc / Back kicks DOM focus off the input so the wrapper's
          // own focus (and therefore arrow-key nav) takes over again.
          if (e.key === 'Escape' || e.key === 'GoBack') {
            e.stopPropagation()
            inputRef.current?.blur()
          }
        }}
        style={styles.input}
        autoComplete={autoComplete}
        autoCorrect="off"
        spellCheck={false}
      />
    </label>
  )
}

function SubmitButton({ busy, onSelect }: { busy: boolean; onSelect: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(busy ? undefined : onSelect)
  return (
    <button
      ref={ref}
      tabIndex={-1}
      type="submit"
      disabled={busy}
      style={{ ...styles.button, ...(busy ? styles.buttonDisabled : {}) }}
    >
      {busy ? 'Signing in…' : 'Sign in'}
    </button>
  )
}

// ── Server picker modal ────────────────────────────────────

function ServerPicker({
  current, onClose, onChoose,
}: {
  current: string
  onClose: () => void
  onChoose: (url: string) => void
}) {
  const [draft, setDraft] = useState('')
  const remembered = api.getRememberedServers()
  // Show "current" at the top of the list even if it hasn't been
  // remembered yet (e.g. typed but not yet logged-in with).
  const list = current && !remembered.includes(current)
    ? [current, ...remembered]
    : remembered

  const save = () => {
    const v = draft.trim().replace(/\/+$/, '')
    if (!v) return
    onChoose(v)
  }

  return (
    <Popup onClose={onClose}>
      <h3 style={styles.popupTitle}>Choose server</h3>

      {list.length === 0 && (
        <p style={styles.popupEmpty}>No servers yet — add one below.</p>
      )}

      <div style={styles.popupList}>
        {list.map(url => (
          <ServerRow
            key={url}
            url={url}
            active={url === current}
            onSelect={() => onChoose(url)}
            onForget={() => { api.forgetServer(url); onClose() }}
          />
        ))}
      </div>

      <div style={styles.addRow}>
        <span style={styles.addLabel}>Add</span>
        <FocusableInput
          label=""
          value={draft}
          onChange={setDraft}
          autoComplete="url"
        />
        <SaveButton onSelect={save} />
      </div>
    </Popup>
  )
}

function ServerRow({
  url, active, onSelect, onForget,
}: {
  url: string
  active: boolean
  onSelect: () => void
  onForget: () => void
}) {
  const selRef = useFocusable<HTMLButtonElement>(onSelect)
  const rmRef = useFocusable<HTMLButtonElement>(onForget)
  return (
    <div style={styles.serverRow}>
      <button
        ref={selRef}
        tabIndex={-1}
        onClick={onSelect}
        style={{ ...styles.serverRowMain, ...(active ? styles.serverRowMainActive : {}) }}
      >
        <span style={styles.serverRowUrl}>{url}</span>
        {active && <span style={styles.serverRowCheck}>✓</span>}
      </button>
      <button ref={rmRef} tabIndex={-1} onClick={onForget} style={styles.serverRowForget} aria-label="Forget">
        ×
      </button>
    </div>
  )
}

function SaveButton({ onSelect }: { onSelect: () => void }) {
  const ref = useFocusable<HTMLButtonElement>(onSelect)
  return (
    <button ref={ref} tabIndex={-1} type="button" onClick={onSelect} style={styles.saveBtn}>
      Save
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    height: '100vh',
    display: 'grid',
    placeItems: 'center',
    background: 'radial-gradient(1200px 800px at 30% 20%, #2a2a2a 0%, #131313 60%)',
  },
  card: {
    width: '36rem',
    padding: '3rem',
    background: 'var(--bg-elev)',
    borderRadius: 'var(--radius-lg)',
    display: 'flex',
    flexDirection: 'column',
    gap: '1.25rem',
  },
  title: {
    fontFamily: 'var(--font-logo)',
    fontSize: '3.25rem',
    margin: 0,
    fontWeight: 700,
    letterSpacing: '0.02em',
  },
  subtitle: {
    margin: 0,
    color: 'var(--text-muted)',
    fontSize: '1.125rem',
  },

  serverBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.75rem',
    background: 'var(--bg-elev-2)',
    color: 'var(--text)',
    borderRadius: 'var(--radius-md)',
    padding: '0.85rem 1.25rem',
    textAlign: 'left',
    width: '100%',
  },
  serverBtnLabel: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
  },
  serverBtnValue: {
    flex: 1,
    minWidth: 0,
    fontSize: '1rem',
    fontWeight: 600,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  serverBtnChevron: {
    color: 'var(--text-muted)',
    fontSize: '1.5rem',
  },

  label: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    color: 'var(--text-muted)',
    fontSize: '0.95rem',
  },
  labelText: {
    color: 'var(--text-muted)',
  },
  input: {
    background: 'var(--bg-elev-2)',
    color: 'var(--text)',
    border: '1px solid transparent',
    borderRadius: 'var(--radius-md)',
    padding: '1rem 1.25rem',
    fontSize: '1.25rem',
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box',
  },
  button: {
    marginTop: '0.5rem',
    background: 'var(--accent)',
    color: 'var(--on-accent)',
    padding: '1rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.25rem',
    fontWeight: 600,
  },
  buttonDisabled: { opacity: 0.5, cursor: 'default' },
  error: {
    background: 'rgba(255, 80, 80, 0.12)',
    color: 'var(--error)',
    padding: '0.75rem 1rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '0.95rem',
  },

  // ── Picker styles ──────────────────────────────────────
  popupTitle: {
    margin: '0 0 1rem',
    fontSize: '1.25rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
  },
  popupEmpty: {
    margin: '0 0 1rem',
    color: 'var(--text-muted)',
    fontSize: '0.95rem',
  },
  popupList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.4rem',
  },
  serverRow: {
    display: 'flex',
    alignItems: 'stretch',
    gap: '0.4rem',
  },
  serverRowMain: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.85rem 1.25rem',
    fontSize: '1rem',
    color: 'var(--text)',
    background: 'transparent',
    borderRadius: 'var(--radius-md)',
    textAlign: 'left',
  },
  serverRowMainActive: {
    background: 'var(--accent-soft)',
    fontWeight: 600,
  },
  serverRowUrl: {
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    flex: 1,
    minWidth: 0,
  },
  serverRowCheck: {
    color: 'var(--accent)',
    marginLeft: '0.75rem',
  },
  serverRowForget: {
    width: '2.75rem',
    color: 'var(--text-muted)',
    background: 'transparent',
    borderRadius: 'var(--radius-md)',
    fontSize: '1.25rem',
  },

  addRow: {
    display: 'flex',
    alignItems: 'flex-end',
    gap: '0.5rem',
    marginTop: '1rem',
    paddingTop: '1rem',
    borderTop: '1px solid rgba(255,255,255,0.08)',
  },
  addLabel: {
    fontSize: '0.85rem',
    color: 'var(--text-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    paddingBottom: '1rem',
  },
  saveBtn: {
    background: 'var(--accent)',
    color: 'var(--on-accent)',
    padding: '0 1.25rem',
    height: '3.5rem',
    borderRadius: 'var(--radius-md)',
    fontSize: '1rem',
    fontWeight: 600,
  },
}

import { type FormEvent, useEffect, useRef, useState } from 'react'
import { RiAddLine, RiCloseLine, RiDeleteBinLine, RiShieldKeyholeLine, RiFileCopyLine, RiCheckLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { APIToken } from '../../api'
import styles from './ApiTokensPage.module.css'

function relativeTime(iso?: string): string {
  if (!iso) return 'never'
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function expiryLabel(iso?: string): { text: string; expired: boolean } {
  if (!iso) return { text: 'never expires', expired: false }
  const when = new Date(iso).getTime()
  if (when < Date.now()) return { text: 'expired', expired: true }
  const days = Math.ceil((when - Date.now()) / 86400000)
  return { text: `expires in ${days}d`, expired: false }
}

export function ApiTokensPage() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [mintOpen, setMintOpen] = useState(false)
  const [minted, setMinted] = useState<{ name: string; token: string } | null>(null)

  const reload = () => {
    setLoading(true)
    api.listAPITokens()
      .then(setTokens)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load API tokens'))
      .finally(() => setLoading(false))
  }

  // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount
  useEffect(() => { reload() }, [])

  const revoke = async (t: APIToken) => {
    if (!confirm(`Revoke the token "${t.name}"? Any integration using it will lose access.`)) return
    try {
      await api.revokeAPIToken(t.id)
      reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to revoke token')
    }
  }

  return (
    <div>
      <div className={styles.header}>
        <div>
          <h1 className="headline-sm">API tokens</h1>
          <p className={`body-sm ${styles.blurb}`}>
            Personal read-only tokens for third-party integrations. Send as
            <code> Authorization: Bearer &lt;token&gt;</code>. A token is shown once at creation —
            copy it then. Revoke any time to cut access immediately.
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setMintOpen(true)}>
          <RiAddLine size={16} /> Mint token
        </button>
      </div>

      {error && <p className={styles.error}>{error}</p>}
      {loading ? (
        <p className="label-sm">Loading…</p>
      ) : tokens.length === 0 ? (
        <div className={styles.empty}>
          <RiShieldKeyholeLine size={22} />
          <span className="body-sm">No API tokens yet. Mint one to give an integration read-only access.</span>
        </div>
      ) : (
        <div className={styles.list}>
          {tokens.map(t => {
            const exp = expiryLabel(t.expires_at)
            return (
              <div key={t.id} className={`surface ${styles.row} ${t.revoked ? styles.revoked : ''}`}>
                <div className={styles.rowMain}>
                  <span className={`label-md ${styles.name}`}>{t.name}</span>
                  <span className={`label-sm ${styles.prefix}`}>{t.token_prefix}…</span>
                  {t.revoked && <span className="badge">revoked</span>}
                </div>
                <div className={styles.scopes}>
                  {t.scopes.map(s => <span key={s} className={`badge ${styles.scope}`}>{s}</span>)}
                </div>
                <span className={`${styles.meta} ${exp.expired ? styles.expired : ''}`}>
                  used {relativeTime(t.last_used_at)} · {exp.text}
                </span>
                {!t.revoked && (
                  <button className="btn btn-icon" onClick={() => revoke(t)} aria-label="Revoke token" title="Revoke">
                    <RiDeleteBinLine size={16} />
                  </button>
                )}
              </div>
            )
          })}
        </div>
      )}

      {mintOpen && (
        <MintModal
          onClose={() => setMintOpen(false)}
          onMinted={(name, token) => { setMintOpen(false); setMinted({ name, token }); reload() }}
        />
      )}

      {minted && <RevealModal name={minted.name} token={minted.token} onClose={() => setMinted(null)} />}
    </div>
  )
}

function MintModal({ onClose, onMinted }: { onClose: () => void; onMinted: (name: string, token: string) => void }) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [name, setName] = useState('')
  const [expiresInDays, setExpiresInDays] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true); setError('')
    try {
      const days = parseInt(expiresInDays, 10)
      const res = await api.mintAPIToken({
        name: name.trim(),
        expires_in_days: Number.isFinite(days) && days > 0 ? days : undefined,
      })
      onMinted(res.api_token.name, res.token)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to mint token')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className={styles.overlay} ref={overlayRef} onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">Mint API token</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <form onSubmit={submit} className={styles.form}>
          <label className={styles.field}>
            <span className="label-sm">Name</span>
            <input
              className="input" value={name} onChange={e => setName(e.target.value)}
              placeholder="Grafana dashboard"
            />
          </label>
          <label className={styles.field}>
            <span className="label-sm">Expires in days <span style={{ opacity: 0.6 }}>(blank = never)</span></span>
            <input
              className="input" value={expiresInDays} onChange={e => setExpiresInDays(e.target.value)}
              type="number" min="1" placeholder="90"
            />
          </label>
          <p className="body-sm" style={{ opacity: 0.7, margin: 0 }}>
            Tokens are read-only (<code>read</code> scope).
          </p>
          {error && <p className={styles.error}>{error}</p>}
          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={submitting || !name.trim()}>
              {submitting ? 'Minting…' : 'Mint'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function RevealModal({ name, token, onClose }: { name: string; token: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    void navigator.clipboard?.writeText(token)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className={styles.overlay} onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">Token for {name}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <div className={styles.form}>
          <p className="body-sm">
            Copy this now — it won't be shown again. Send it as
            <code> Authorization: Bearer &lt;token&gt;</code>.
          </p>
          <div className={styles.keyReveal}>
            <code className={styles.keyText}>{token}</code>
            <button className="btn btn-icon" onClick={copy} aria-label="Copy token" title="Copy">
              {copied ? <RiCheckLine size={16} /> : <RiFileCopyLine size={16} />}
            </button>
          </div>
          <div className={styles.actions}>
            <button className="btn btn-primary" onClick={onClose}>Done</button>
          </div>
        </div>
      </div>
    </div>
  )
}

import { type FormEvent, useEffect, useRef, useState } from 'react'
import { RiAddLine, RiCloseLine, RiDeleteBinLine, RiKey2Line, RiFileCopyLine, RiCheckLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { ServiceKey } from '../../api'
import { SERVICE_KEY_SCOPES, SERVICE_KEY_DEFAULTS } from '../../api/types'
import styles from './ServiceKeysPage.module.css'

const KNOWN_SERVICES = Object.keys(SERVICE_KEY_DEFAULTS)

function relativeTime(iso?: string): string {
  if (!iso) return 'never'
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export function ServiceKeysPage() {
  const [keys, setKeys] = useState<ServiceKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [mintOpen, setMintOpen] = useState(false)
  const [minted, setMinted] = useState<{ name: string; key: string } | null>(null)

  const reload = () => {
    setLoading(true)
    api.listServiceKeys()
      .then(setKeys)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load service keys'))
      .finally(() => setLoading(false))
  }

  // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount
  useEffect(() => { reload() }, [])

  const revoke = async (k: ServiceKey) => {
    if (!confirm(`Revoke the key for "${k.name}"? The service will fail auth until re-keyed.`)) return
    try {
      await api.revokeServiceKey(k.id)
      reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to revoke key')
    }
  }

  return (
    <div>
      <div className={styles.header}>
        <div>
          <h1 className="headline-sm">Service keys</h1>
          <p className={`body-sm ${styles.blurb}`}>
            Per-service API keys for the internal services (scan / trans / meta). A key is shown
            once at creation — copy it into that service's <code>RIVER_API_KEY</code> and restart it.
            Services without a key fall back to password login.
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setMintOpen(true)}>
          <RiAddLine size={16} /> Mint key
        </button>
      </div>

      {error && <p className={styles.error}>{error}</p>}
      {loading ? (
        <p className="label-sm">Loading…</p>
      ) : keys.length === 0 ? (
        <div className={styles.empty}>
          <RiKey2Line size={22} />
          <span className="body-sm">No service keys yet. Services are using password auth.</span>
        </div>
      ) : (
        <div className={styles.list}>
          {keys.map(k => (
            <div key={k.id} className={`surface ${styles.row} ${k.revoked ? styles.revoked : ''}`}>
              <div className={styles.rowMain}>
                <span className={`label-md ${styles.name}`}>{k.name}</span>
                <span className={`label-sm ${styles.prefix}`}>{k.key_prefix}…</span>
                {k.revoked && <span className="badge">revoked</span>}
              </div>
              <div className={styles.scopes}>
                {k.scopes.map(s => <span key={s} className={`badge ${styles.scope}`}>{s}</span>)}
              </div>
              <span className={`label-sm ${styles.used}`}>used {relativeTime(k.last_used_at)}</span>
              {!k.revoked && (
                <button className="btn btn-icon" onClick={() => revoke(k)} aria-label="Revoke key" title="Revoke">
                  <RiDeleteBinLine size={16} />
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {mintOpen && (
        <MintModal
          onClose={() => setMintOpen(false)}
          onMinted={(name, key) => { setMintOpen(false); setMinted({ name, key }); reload() }}
        />
      )}

      {minted && <RevealModal name={minted.name} keyValue={minted.key} onClose={() => setMinted(null)} />}
    </div>
  )
}

function MintModal({ onClose, onMinted }: { onClose: () => void; onMinted: (name: string, key: string) => void }) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  // When the name matches a known service, pre-fill its recommended scopes.
  const pickName = (v: string) => {
    setName(v)
    if (SERVICE_KEY_DEFAULTS[v]) setScopes(SERVICE_KEY_DEFAULTS[v])
  }

  const toggle = (s: string) =>
    setScopes(prev => prev.includes(s) ? prev.filter(x => x !== s) : [...prev, s])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true); setError('')
    try {
      const res = await api.mintServiceKey(name.trim(), scopes)
      onMinted(res.service_key.name, res.key)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to mint key')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className={styles.overlay} ref={overlayRef} onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">Mint service key</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <form onSubmit={submit} className={styles.form}>
          <label className={styles.field}>
            <span className="label-sm">Service name</span>
            <input
              className="input" value={name} onChange={e => pickName(e.target.value)}
              placeholder="river-meta-movie" list="known-services" autoCapitalize="off" autoCorrect="off" spellCheck={false}
            />
            <datalist id="known-services">
              {KNOWN_SERVICES.map(s => <option key={s} value={s} />)}
            </datalist>
          </label>
          <div className={styles.field}>
            <span className="label-sm">Scopes</span>
            <div className={styles.scopeGrid}>
              {SERVICE_KEY_SCOPES.map(s => (
                <label key={s} className={styles.scopeCheck}>
                  <input type="checkbox" checked={scopes.includes(s)} onChange={() => toggle(s)} />
                  <span className="label-sm">{s}</span>
                </label>
              ))}
            </div>
          </div>
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

function RevealModal({ name, keyValue, onClose }: { name: string; keyValue: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    void navigator.clipboard?.writeText(keyValue)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className={styles.overlay} onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">Key for {name}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <div className={styles.form}>
          <p className="body-sm">
            Copy this now — it won't be shown again. Set it as <code>RIVER_API_KEY</code> for
            <strong> {name}</strong> and restart the service.
          </p>
          <div className={styles.keyReveal}>
            <code className={styles.keyText}>{keyValue}</code>
            <button className="btn btn-icon" onClick={copy} aria-label="Copy key" title="Copy">
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

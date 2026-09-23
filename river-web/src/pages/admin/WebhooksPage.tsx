import { type FormEvent, useEffect, useRef, useState } from 'react'
import { RiAddLine, RiCloseLine, RiDeleteBinLine, RiWebhookLine, RiFileCopyLine, RiCheckLine, RiEdit2Line, RiPulseLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { Webhook, WebhookDelivery } from '../../api'
import { WEBHOOK_EVENT_KINDS } from '../../api/types'
import styles from './WebhooksPage.module.css'

function relativeTime(iso?: string): string {
  if (!iso) return 'never'
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export function WebhooksPage() {
  const [hooks, setHooks] = useState<Webhook[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Webhook | null>(null)
  const [revealed, setRevealed] = useState<{ name: string; secret: string } | null>(null)
  const [openDeliveries, setOpenDeliveries] = useState<string | null>(null)

  const reload = () => {
    setLoading(true)
    api.listWebhooks()
      .then(setHooks)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load webhooks'))
      .finally(() => setLoading(false))
  }

  // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount
  useEffect(() => { reload() }, [])

  const remove = async (w: Webhook) => {
    if (!confirm(`Delete the webhook "${w.name}"? Delivery to ${w.url} will stop.`)) return
    try {
      await api.deleteWebhook(w.id)
      reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to delete webhook')
    }
  }

  return (
    <div>
      <div className={styles.header}>
        <div>
          <h1 className="headline-sm">Webhooks</h1>
          <p className={`body-sm ${styles.blurb}`}>
            Notify a third-party service when media completes its lifecycle. Each event is POSTed
            as JSON and signed with an HMAC-SHA256 <code>X-River-Signature</code> header (the
            signing secret is shown once at creation). Leave events unselected to receive all kinds.
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => { setEditing(null); setFormOpen(true) }}>
          <RiAddLine size={16} /> Add webhook
        </button>
      </div>

      {error && <p className={styles.error}>{error}</p>}
      {loading ? (
        <p className="label-sm">Loading…</p>
      ) : hooks.length === 0 ? (
        <div className={styles.empty}>
          <RiWebhookLine size={22} />
          <span className="body-sm">No webhooks yet. Add one to notify an external service on media events.</span>
        </div>
      ) : (
        <div className={styles.list}>
          {hooks.map(w => (
            <div key={w.id} className={`surface ${styles.row} ${w.enabled ? '' : styles.disabled}`}>
              <div className={styles.rowMain}>
                <div className={styles.nameLine}>
                  <span className={`label-md ${styles.name}`}>{w.name}</span>
                  {!w.enabled && <span className="badge">disabled</span>}
                </div>
                <span className={styles.url}>{w.url}</span>
                {w.last_error && <span className={styles.lastError} title={w.last_error}>last error: {w.last_error}</span>}
              </div>
              <div className={styles.events}>
                {w.events.length === 0
                  ? <span className={`badge ${styles.event}`}>all events</span>
                  : w.events.map(e => <span key={e} className={`badge ${styles.event}`}>{e}</span>)}
              </div>
              <div className={styles.rowActions}>
                <button
                  className="btn btn-icon"
                  onClick={() => setOpenDeliveries(openDeliveries === w.id ? null : w.id)}
                  aria-label="View deliveries" title="Recent deliveries"
                >
                  <RiPulseLine size={16} />
                </button>
                <button className="btn btn-icon" onClick={() => { setEditing(w); setFormOpen(true) }} aria-label="Edit webhook" title="Edit">
                  <RiEdit2Line size={16} />
                </button>
                <button className="btn btn-icon" onClick={() => remove(w)} aria-label="Delete webhook" title="Delete">
                  <RiDeleteBinLine size={16} />
                </button>
              </div>
              {openDeliveries === w.id && <DeliveriesDrawer webhookID={w.id} />}
            </div>
          ))}
        </div>
      )}

      {formOpen && (
        <WebhookFormModal
          existing={editing}
          onClose={() => setFormOpen(false)}
          onSaved={(name, secret) => {
            setFormOpen(false)
            if (secret) setRevealed({ name, secret })
            reload()
          }}
        />
      )}

      {revealed && <RevealModal name={revealed.name} secret={revealed.secret} onClose={() => setRevealed(null)} />}
    </div>
  )
}

function DeliveriesDrawer({ webhookID }: { webhookID: string }) {
  const [deliveries, setDeliveries] = useState<WebhookDelivery[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api.listWebhookDeliveries(webhookID)
      .then(setDeliveries)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load deliveries'))
  }, [webhookID])

  if (error) return <div className={styles.deliveries}><span className={styles.error}>{error}</span></div>
  if (!deliveries) return <div className={styles.deliveries}><span className="label-sm">Loading deliveries…</span></div>
  if (deliveries.length === 0) return <div className={styles.deliveries}><span className="label-sm">No deliveries yet.</span></div>

  return (
    <div className={styles.deliveries} style={{ gridColumn: '1 / -1' }}>
      {deliveries.map(d => (
        <div key={d.id} className={styles.delivery}>
          <span className={d.status === 'delivered' ? styles.statusOk : styles.statusFail}>{d.status}</span>
          <span className={styles.deliveryEvent} title={d.event}>{d.event}</span>
          <span>{d.response_code ? `HTTP ${d.response_code}` : d.error || '—'}{d.attempts > 1 ? ` · ${d.attempts} attempts` : ''}</span>
          <span>{relativeTime(d.created_at)}</span>
        </div>
      ))}
    </div>
  )
}

function WebhookFormModal({ existing, onClose, onSaved }: {
  existing: Webhook | null
  onClose: () => void
  onSaved: (name: string, secret?: string) => void
}) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [name, setName] = useState(existing?.name ?? '')
  const [url, setUrl] = useState(existing?.url ?? '')
  const [events, setEvents] = useState<string[]>(existing?.events ?? [])
  const [enabled, setEnabled] = useState(existing?.enabled ?? true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const toggle = (e: string) =>
    setEvents(prev => prev.includes(e) ? prev.filter(x => x !== e) : [...prev, e])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true); setError('')
    try {
      if (existing) {
        await api.updateWebhook(existing.id, { name: name.trim(), url: url.trim(), events, enabled })
        onSaved(name.trim())
      } else {
        const res = await api.createWebhook({ name: name.trim(), url: url.trim(), events, enabled })
        onSaved(res.webhook.name, res.secret)
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save webhook')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className={styles.overlay} ref={overlayRef} onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">{existing ? 'Edit webhook' : 'Add webhook'}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <form onSubmit={submit} className={styles.form}>
          <label className={styles.field}>
            <span className="label-sm">Name</span>
            <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Home Assistant" />
          </label>
          <label className={styles.field}>
            <span className="label-sm">Payload URL</span>
            <input
              className="input" value={url} onChange={e => setUrl(e.target.value)}
              placeholder="https://example.com/hooks/river" type="url"
              autoCapitalize="off" autoCorrect="off" spellCheck={false}
            />
          </label>
          <div className={styles.field}>
            <span className="label-sm">Events <span className="label-sm" style={{ opacity: 0.6 }}>(none = all)</span></span>
            <div className={styles.eventGrid}>
              {WEBHOOK_EVENT_KINDS.map(ev => (
                <label key={ev} className={styles.eventCheck}>
                  <input type="checkbox" checked={events.includes(ev)} onChange={() => toggle(ev)} />
                  <span className="label-sm">{ev}</span>
                </label>
              ))}
            </div>
          </div>
          <label className={styles.checkRow}>
            <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
            <span className="label-sm">Enabled</span>
          </label>
          {error && <p className={styles.error}>{error}</p>}
          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={submitting || !name.trim() || !url.trim()}>
              {submitting ? 'Saving…' : existing ? 'Save' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function RevealModal({ name, secret, onClose }: { name: string; secret: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    void navigator.clipboard?.writeText(secret)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className={styles.overlay} onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.dialogHead}>
          <h2 className="headline-sm">Signing secret for {name}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close"><RiCloseLine size={20} /></button>
        </div>
        <div className={styles.form}>
          <p className="body-sm">
            Copy this now — it won't be shown again. Use it to verify the
            <code> X-River-Signature</code> header (HMAC-SHA256 of the raw request body).
          </p>
          <div className={styles.keyReveal}>
            <code className={styles.keyText}>{secret}</code>
            <button className="btn btn-icon" onClick={copy} aria-label="Copy secret" title="Copy">
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

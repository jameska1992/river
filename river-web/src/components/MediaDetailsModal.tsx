import { useEffect, useRef, useState } from 'react'
import { RiCloseLine, RiFileCopyLine, RiCheckLine } from 'react-icons/ri'
import styles from './MediaDetailsModal.module.css'

interface Props {
  title: string
  // Free-form record. Whatever the detail page hands in gets rendered;
  // typically the raw entity (Movie, TVShow, Album, Audiobook, Episode).
  item: Record<string, unknown>
  // Optional ordered list of keys to show first. Anything else falls to
  // the bottom, alphabetized. Use this to surface source_path/file_path
  // ahead of less interesting fields without having to enumerate the lot.
  primaryKeys?: string[]
  // Keys to hide entirely. The default list strips heavy nested objects
  // and arrays that the parent has already rendered visually (seasons[]
  // on a TV show, tracks[] on an album, etc.) — they'd just be JSON noise
  // in this view.
  hideKeys?: string[]
  onClose: () => void
}

const DEFAULT_HIDE = new Set([
  'seasons', 'episodes', 'tracks', 'chapters', 'albums',
])

// ISO 8601 timestamp shape — catches both Go's default time.RFC3339 and
// values with a fractional-seconds component.
const isoDateRe = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})$/

// Path-or-URL heuristic for deciding when to show a copy button. We want
// it on filesystem paths (/media/...) and URLs (http://, https://) since
// admins typically grab those to paste into a terminal or browser.
function looksLikePathOrURL(v: string): boolean {
  return v.startsWith('/') || /^https?:\/\//.test(v) || /^[A-Z]:\\/.test(v)
}

export function MediaDetailsModal({ title, item, primaryKeys, hideKeys, onClose }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const hide = new Set([...DEFAULT_HIDE, ...(hideKeys ?? [])])
  const allKeys = Object.keys(item).filter(k => !hide.has(k))
  const primary = (primaryKeys ?? []).filter(k => allKeys.includes(k))
  const rest = allKeys.filter(k => !primary.includes(k)).sort()
  const ordered = [...primary, ...rest]

  return (
    <div
      className={styles.overlay}
      ref={overlayRef}
      onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.header}>
          <h2 className={`headline-sm ${styles.headerTitle}`}>{title}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <dl className={styles.list}>
          {ordered.map(key => (
            <div key={key} className={styles.row}>
              <dt className={`label-sm ${styles.key}`}>{formatLabel(key)}</dt>
              <dd className={styles.value}><ValueCell value={item[key]} /></dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  )
}

function ValueCell({ value }: { value: unknown }) {
  if (value === null || value === undefined || value === '') {
    return <span className={styles.empty}>—</span>
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return <span className={styles.empty}>—</span>
    return <span>{value.map(v => typeof v === 'object' ? JSON.stringify(v) : String(v)).join(', ')}</span>
  }
  if (typeof value === 'object') {
    return <pre className={styles.json}>{JSON.stringify(value, null, 2)}</pre>
  }
  if (typeof value === 'boolean') {
    return <span>{value ? 'Yes' : 'No'}</span>
  }
  if (typeof value === 'number') {
    return <span>{value}</span>
  }
  const s = String(value)
  if (isoDateRe.test(s)) {
    let formatted: string | null = null
    try {
      formatted = new Date(s).toLocaleString()
    } catch { /* fall through to plain string */ }
    if (formatted !== null) {
      return <span title={s}>{formatted}</span>
    }
  }
  if (looksLikePathOrURL(s)) {
    return <CopyableText text={s} />
  }
  return <span>{s}</span>
}

// CopyableText renders a path/URL alongside a copy-to-clipboard button.
// The button flips to a checkmark briefly so the click feels acknowledged
// without resorting to a toast system.
function CopyableText({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1200)
    } catch {
      // Clipboard API requires secure context or user gesture; failing
      // silently is fine — the text is still visible and selectable.
    }
  }
  return (
    <span className={styles.copyableWrap}>
      <code className={styles.code}>{text}</code>
      <button
        type="button"
        className={`btn btn-icon ${styles.copyBtn}`}
        onClick={handleCopy}
        aria-label={copied ? 'Copied' : 'Copy to clipboard'}
        title={copied ? 'Copied' : 'Copy to clipboard'}
      >
        {copied ? <RiCheckLine size={14} /> : <RiFileCopyLine size={14} />}
      </button>
    </span>
  )
}

function formatLabel(key: string): string {
  return key.split('_').map(w => (w.length === 0 ? w : w[0].toUpperCase() + w.slice(1))).join(' ')
}

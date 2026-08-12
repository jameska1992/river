import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { Audiobook } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

interface Props {
  book: Pick<Audiobook, 'id' | 'title' | 'open_library_key'>
  onClose: () => void
  onSubmitted?: () => void
}

// normalizeWorkKey mirrors the server: accepts a full URL, a "/works/OL…W"
// path, or a bare "OL…W", and returns the canonical "/works/OL…W" (or null).
function normalizeWorkKey(raw: string): string | null {
  let s = raw.trim()
  s = s.replace(/^https?:\/\/openlibrary\.org/, '')
  s = s.replace(/\/$/, '')
  s = s.replace(/^\/works\//, '')
  return /^OL\d+W$/.test(s) ? `/works/${s}` : null
}

// IdentifyAudiobookModal: admin override that pins the book to a specific Open
// Library work. Once set, re-enrichment resolves by this key instead of a
// title search, so a bad match can be corrected and won't drift back.
export function IdentifyAudiobookModal({ book, onClose, onSubmitted }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [key, setKey] = useState(book.open_library_key ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    const normalized = normalizeWorkKey(key)
    if (!normalized) {
      setError('Enter an Open Library work key like OL45804W or /works/OL45804W')
      return
    }
    setSubmitting(true)
    try {
      await api.identifyAudiobook(book.id, { open_library_key: normalized })
      onSubmitted?.()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to identify audiobook')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className={styles.overlay}
      ref={overlayRef}
      onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.header}>
          <h2 className={`headline-sm ${styles.headerTitle}`}>Identify audiobook</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>
            <p className="body-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
              Pin this book to a specific{' '}
              <a href="https://openlibrary.org" target="_blank" rel="noreferrer" style={{ color: 'var(--color-primary)' }}>
                Open Library
              </a>{' '}
              work. Metadata refresh then resolves by this key instead of searching by title, so
              the match sticks. Find the key in the work's URL (e.g. openlibrary.org<b>/works/OL45804W</b>).
            </p>

            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>
                Work key <span className={styles.hint}>(e.g. OL45804W)</span>
              </span>
              <input
                className={styles.input}
                value={key}
                onChange={e => setKey(e.target.value)}
                placeholder="OL45804W"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
              />
            </label>
          </div>

          {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={submitting}>
              {submitting ? 'Identifying…' : 'Identify & refresh'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

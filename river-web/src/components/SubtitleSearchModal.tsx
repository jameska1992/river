import { useCallback, useEffect, useRef, useState } from 'react'
import { RiCloseLine, RiCheckLine } from 'react-icons/ri'
import type { SubtitleSearchResult } from '../api'
import { ApiError } from '../api'
import styles from './MetadataModal.module.css'

interface Props {
  title: string
  // onSearch/onAttach are supplied by the caller so this modal works for both
  // movies and episodes without knowing the media type.
  onSearch: (languages: string) => Promise<SubtitleSearchResult[]>
  onAttach: (r: SubtitleSearchResult) => Promise<void>
  onClose: () => void
  // onAttached fires after a successful attach so the caller can refresh its
  // subtitle list.
  onAttached?: () => void
}

// SubtitleSearchModal searches SubDL for a title's subtitles and attaches a
// chosen result. Stays open after an attach so several can be added in a row.
export function SubtitleSearchModal({ title, onSearch, onAttach, onClose, onAttached }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const [languages, setLanguages] = useState('EN')
  const [results, setResults] = useState<SubtitleSearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const [error, setError] = useState('')
  const [attachingUrl, setAttachingUrl] = useState('')
  const [attachedUrls, setAttachedUrls] = useState<Set<string>>(new Set())

  const runSearch = useCallback(async (langs: string) => {
    setLoading(true)
    setError('')
    try {
      setResults(await onSearch(langs.trim()))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Search failed.')
      setResults([])
    } finally {
      setLoading(false)
      setSearched(true)
    }
  }, [onSearch])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial search on open
    void runSearch(languages)
    // Intentionally only on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const attach = async (r: SubtitleSearchResult) => {
    setAttachingUrl(r.url)
    setError('')
    try {
      await onAttach(r)
      setAttachedUrls(prev => new Set(prev).add(r.url))
      onAttached?.()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to attach subtitle.')
    } finally {
      setAttachingUrl('')
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
          <h2 className={`headline-sm ${styles.headerTitle}`}>Search subtitles — {title}</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form
          className={styles.form}
          onSubmit={e => { e.preventDefault(); void runSearch(languages) }}
        >
          <div className={styles.fields}>
            <label className={styles.field}>
              <span className={`label-sm ${styles.label}`}>
                Languages <span className={styles.hint}>(comma-separated codes, e.g. EN,FR)</span>
              </span>
              <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                <input
                  className={styles.input}
                  value={languages}
                  onChange={e => setLanguages(e.target.value)}
                  placeholder="EN"
                  autoCapitalize="characters"
                  autoCorrect="off"
                  spellCheck={false}
                  style={{ flex: 1 }}
                />
                <button type="submit" className="btn btn-primary" disabled={loading}>
                  {loading ? 'Searching…' : 'Search'}
                </button>
              </div>
            </label>

            {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

            {!loading && searched && results.length === 0 && !error && (
              <p className="body-sm" style={{ color: 'var(--color-on-surface-variant)' }}>
                No subtitles found for these languages.
              </p>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '46vh', overflowY: 'auto' }}>
              {results.map((r, i) => {
                const added = attachedUrls.has(r.url)
                const busy = attachingUrl === r.url
                return (
                  <div
                    key={`${r.url}-${i}`}
                    className="surface"
                    style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', padding: '10px var(--space-2)', borderRadius: 'var(--radius-md)' }}
                  >
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div className="label-md" style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {r.release_name || r.name || 'Subtitle'}
                      </div>
                      <div className="label-sm" style={{ color: 'var(--color-on-surface-variant)', display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                        <span>{r.language || r.lang}</span>
                        {r.author && <span>· {r.author}</span>}
                        {r.hi && <span className="badge">HI</span>}
                      </div>
                    </div>
                    <button
                      className={`btn ${added ? '' : 'btn-primary'}`}
                      disabled={busy || added}
                      onClick={() => void attach(r)}
                      style={{ flexShrink: 0 }}
                    >
                      {added ? <><RiCheckLine size={16} /> Added</> : busy ? 'Adding…' : 'Add'}
                    </button>
                  </div>
                )
              })}
            </div>
          </div>

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose}>Done</button>
          </div>
        </form>
      </div>
    </div>
  )
}

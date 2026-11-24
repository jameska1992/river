import { useEffect, useId, useRef, useState } from 'react'
import { RiLinkUnlink, RiUserLine } from 'react-icons/ri'
import type { ExternalPersonResult } from '../api'
import { api, ApiError } from '../api'
import { imageUrl } from '../util/imageUrl'
import styles from './PersonSearchInput.module.css'

// Typing pauses this long before a TMDB search fires, and short queries are
// never sent — keeps the picker from spending TMDB quota on every keystroke.
export const SEARCH_DEBOUNCE_MS = 300
export const MIN_QUERY_LENGTH = 2

interface Props {
  value: string
  // Non-null when the row is linked to a TMDB person.
  tmdbId: number | null
  // Free-text edit. Typing over a linked name drops the link — the row is
  // now someone River can't vouch for.
  onChange: (name: string) => void
  onSelect: (person: ExternalPersonResult) => void
  onUnlink: () => void
}

// Name field for the credits editor with TMDB search-as-you-type (#254).
// Picking a hit links the row to that TMDB person; free text still works
// when there's no match (or TMDB is unavailable).
export function PersonSearchInput({ value, tmdbId, onChange, onSelect, onUnlink }: Props) {
  const listId = useId()
  const [query, setQuery] = useState('') // what the user last typed; '' until they do
  const [results, setResults] = useState<ExternalPersonResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  // Drops responses for a query the user has already typed past.
  const latest = useRef(0)
  const listRef = useRef<HTMLUListElement>(null)

  useEffect(() => () => clearTimeout(timer.current), [])

  const search = (raw: string) => {
    clearTimeout(timer.current)
    const seq = ++latest.current
    const q = raw.trim()
    if (q.length < MIN_QUERY_LENGTH) {
      setResults([])
      setLoading(false)
      setError(null)
      return
    }
    setLoading(true)
    timer.current = setTimeout(async () => {
      try {
        const hits = await api.searchTMDBPeople(q)
        if (seq !== latest.current) return
        setResults(hits)
        setError(null)
        setActive(-1) // nothing pre-picked: Enter on a typed name must not link it
      } catch (err) {
        if (seq !== latest.current) return
        setResults([])
        setError(err instanceof ApiError && err.status === 503
          ? 'TMDB search isn’t available (check the TMDB key and metadata service).'
          : 'TMDB search failed. You can still type a name.')
      } finally {
        if (seq === latest.current) setLoading(false)
      }
    }, SEARCH_DEBOUNCE_MS)
  }

  const choose = (p: ExternalPersonResult) => {
    clearTimeout(timer.current)
    latest.current++
    onSelect(p)
    setOpen(false)
    setQuery('')
    setResults([])
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!open || results.length === 0) {
      if (e.key === 'Escape' && open) {
        e.stopPropagation() // close the dropdown, not the modal
        setOpen(false)
      }
      return
    }
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setActive(i => (i + 1) % results.length)
        break
      case 'ArrowUp':
        e.preventDefault()
        setActive(i => (i <= 0 ? results.length - 1 : i - 1))
        break
      case 'Enter':
        if (active >= 0) {
          e.preventDefault() // pick, don't submit the form
          choose(results[active])
        }
        break
      case 'Escape':
        e.stopPropagation()
        setOpen(false)
        break
    }
  }

  const showList = open && query.trim().length >= MIN_QUERY_LENGTH
  const activeId = showList && active >= 0 && results[active] ? `${listId}-${active}` : undefined

  // The editor's rows scroll, and new rows go at the bottom — so without this
  // the list opens below the fold and only the first hit or two shows.
  useEffect(() => {
    if (showList) listRef.current?.scrollIntoView?.({ block: 'nearest' })
  }, [showList, results.length])

  return (
    <div className={styles.wrap}>
      <div className={styles.inputRow}>
        <input
          className={styles.input}
          value={value}
          onChange={e => {
            onChange(e.target.value)
            setQuery(e.target.value)
            setOpen(true)
            search(e.target.value)
          }}
          onKeyDown={handleKeyDown}
          onBlur={() => setOpen(false)}
          onFocus={() => { if (query) setOpen(true) }}
          placeholder="Name (type to search TMDB)"
          aria-label="Name"
          role="combobox"
          aria-autocomplete="list"
          aria-expanded={showList}
          aria-controls={listId}
          aria-activedescendant={activeId}
          autoComplete="off"
        />
        {tmdbId !== null && (
          <>
            <span className={styles.badge} title={`Linked to TMDB person ${tmdbId}`}>TMDB</span>
            <button
              type="button"
              className={`btn btn-icon ${styles.unlink}`}
              onClick={onUnlink}
              aria-label="Unlink from TMDB"
              title="Unlink from TMDB"
            >
              <RiLinkUnlink size={14} />
            </button>
          </>
        )}
      </div>

      {showList && (
        <ul className={styles.list} ref={listRef} id={listId} role="listbox" aria-label="TMDB people">
          {loading && results.length === 0 && <li className={styles.status}>Searching TMDB…</li>}
          {error && <li className={`${styles.status} ${styles.error}`}>{error}</li>}
          {!loading && !error && results.length === 0 && (
            <li className={styles.status}>No TMDB matches. Keep typing to add them by name.</li>
          )}
          {results.map((p, i) => (
            <li
              key={p.tmdb_id}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              className={`${styles.option} ${i === active ? styles.optionActive : ''}`}
              // mousedown, not click: fires before the input's blur closes the list.
              onMouseDown={e => { e.preventDefault(); choose(p) }}
              // mousemove, not mouseenter: the list opening (or scrolling into
              // view) under a resting pointer must not pre-pick a hit for Enter.
              onMouseMove={() => { if (active !== i) setActive(i) }}
            >
              <span className={styles.thumb}>
                {imageUrl(p.profile_path) ? <img src={imageUrl(p.profile_path)} alt="" /> : <RiUserLine size={16} />}
              </span>
              <span className={styles.meta}>
                <span className={styles.name}>
                  {p.name}
                  {p.person_id && <span className={styles.inLibrary}>In library</span>}
                </span>
                <span className={styles.sub}>
                  {[p.known_for_department, p.known_for.join(', ')].filter(Boolean).join(' · ')}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

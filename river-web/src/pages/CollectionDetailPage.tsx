import { useEffect, useState, useRef, useCallback, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  RiAddLine, RiPencilLine, RiDeleteBinLine, RiCloseLine,
  RiFilmLine, RiTv2Line, RiHeadphoneLine, RiCheckLine, RiArrowLeftLine,
} from 'react-icons/ri'
import { api } from '../api'
import type { CollectionDetail, CollectionItem, SearchResultItem, SortOrder } from '../api'
import { MediaCard } from '../components/MediaCard'
import { SortControl, type SortOption } from '../components/SortControl'
import { imageUrl } from '../util/imageUrl'
import styles from './CollectionDetailPage.module.css'

interface SortState { field: string; order: SortOrder }

function sortStorageKey(collectionId: string) {
  return `river:collectionSort:${collectionId}`
}

function loadSort(collectionId: string, fallback: SortState): SortState {
  try {
    const raw = localStorage.getItem(sortStorageKey(collectionId))
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as Partial<SortState>
    return {
      field: typeof parsed.field === 'string' && parsed.field ? parsed.field : fallback.field,
      order: parsed.order === 'desc' ? 'desc' : 'asc',
    }
  } catch {
    return fallback
  }
}

function useSortPref(collectionId: string, fallback: SortState) {
  const [state, setState] = useState<SortState>(() => loadSort(collectionId, fallback))
  useEffect(() => { setState(loadSort(collectionId, fallback)) }, [collectionId]) // eslint-disable-line react-hooks/exhaustive-deps
  const setSort = useCallback((field: string, order: SortOrder) => {
    const next = { field, order }
    setState(next)
    try { localStorage.setItem(sortStorageKey(collectionId), JSON.stringify(next)) } catch { /* ignore */ }
  }, [collectionId])
  return [state, setSort] as const
}

const COLLECTION_SORT_OPTIONS: SortOption[] = [
  { value: 'manual:asc',  label: 'Manual order'        },
  { value: 'title:asc',   label: 'Title (A–Z)'         },
  { value: 'title:desc',  label: 'Title (Z–A)'         },
  { value: 'year:desc',   label: 'Year (newest first)' },
  { value: 'year:asc',    label: 'Year (oldest first)' },
  { value: 'added:desc',  label: 'Recently added'      },
  { value: 'added:asc',   label: 'Oldest added'        },
]

// titleSortKey normalises a title for ordering — must mirror the
// server-side titleSortExpr used by the library list endpoints:
//   1. lowercase
//   2. strip punctuation (keep letters/digits/whitespace) so `"Foo"`
//      and `Foo` sort identically and a leading quote/bracket doesn't
//      push the title to the top
//   3. collapse surrounding whitespace
//   4. strip a leading English article so "The Matrix" sits with
//      "Matrix" and "A Beautiful Mind" sorts under B
function titleSortKey(s: string): string {
  return s.toLowerCase()
    .replace(/[^\p{L}\p{N}\s]/gu, '')
    .trim()
    .replace(/^(the|a|an)\s+/, '')
}

function sortItems(items: CollectionItem[], field: string, order: SortOrder): CollectionItem[] {
  const sorted = [...items]
  const dir = order === 'desc' ? -1 : 1
  switch (field) {
    case 'title':
      sorted.sort((a, b) => dir * titleSortKey(a.title).localeCompare(titleSortKey(b.title), undefined, { sensitivity: 'base' }))
      break
    case 'year':
      // Items without a year sink to the end regardless of direction so a
      // year sort doesn't bury everything with a missing release date.
      sorted.sort((a, b) => {
        const ay = a.year ?? null
        const by = b.year ?? null
        if (ay === null && by === null) return 0
        if (ay === null) return 1
        if (by === null) return -1
        return dir * (ay - by)
      })
      break
    case 'added':
      sorted.sort((a, b) => dir * a.created_at.localeCompare(b.created_at))
      break
    case 'manual':
    default:
      sorted.sort((a, b) => dir * (a.sort_order - b.sort_order))
      break
  }
  return sorted
}

export function CollectionDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [collection, setCollection] = useState<CollectionDetail | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [sort, setSort] = useSortPref(id ?? '', { field: 'manual', order: 'asc' })

  // Edit state
  const [showEdit, setShowEdit] = useState(false)
  const [editName, setEditName] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [saving, setSaving] = useState(false)

  // Add media state
  const [showAddMedia, setShowAddMedia] = useState(false)

  const sortedItems = useMemo(
    () => collection ? sortItems(collection.items, sort.field, sort.order) : [],
    [collection, sort.field, sort.order],
  )

  useEffect(() => {
    if (!id) return
    api.getCollection(id)
      .then(setCollection)
      .catch(() => setError('Collection not found'))
      .finally(() => setIsLoading(false))
  }, [id])

  async function handleDelete() {
    if (!collection) return
    if (!confirm(`Delete "${collection.name}"? This cannot be undone.`)) return
    await api.deleteCollection(collection.id)
    navigate('/collections')
  }

  async function handleSaveEdit(e: React.FormEvent) {
    e.preventDefault()
    if (!collection || !editName.trim()) return
    setSaving(true)
    try {
      const updated = await api.updateCollection(collection.id, { name: editName.trim(), description: editDesc.trim() })
      setCollection(prev => prev ? { ...prev, ...updated } : prev)
      setShowEdit(false)
    } finally {
      setSaving(false)
    }
  }

  function openEdit() {
    if (!collection) return
    setEditName(collection.name)
    setEditDesc(collection.description)
    setShowEdit(true)
  }

  function handleItemAdded(item: CollectionItem) {
    setCollection(prev => prev ? { ...prev, items: [...prev.items, item] } : prev)
  }

  function handleItemRemoved(itemId: string) {
    setCollection(prev => prev ? { ...prev, items: prev.items.filter(i => i.id !== itemId) } : prev)
  }

  if (isLoading) return <LoadingSkeleton />
  if (error || !collection) {
    return (
      <div className={`container ${styles.page}`}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error || 'Not found'}</p>
      </div>
    )
  }

  return (
    <div className={`container ${styles.page}`}>
      {/* Header */}
      <div className={styles.header}>
        <button className={styles.backBtn} onClick={() => navigate('/collections')} aria-label="Back to collections">
          <RiArrowLeftLine size={18} />
          Collections
        </button>
        <div className={styles.headerMeta}>
          <h1 className="headline-lg" style={{ margin: 0 }}>{collection.name}</h1>
          {collection.description && (
            <p className={`body-md ${styles.description}`}>{collection.description}</p>
          )}
        </div>
        <div className={styles.headerActions}>
          <button className={`btn btn-outline ${styles.iconBtn}`} onClick={openEdit} aria-label="Edit collection">
            <RiPencilLine size={16} />
            Edit
          </button>
          <button className={`btn btn-outline ${styles.iconBtnDanger}`} onClick={handleDelete} aria-label="Delete collection">
            <RiDeleteBinLine size={16} />
          </button>
        </div>
      </div>

      {/* Add media button + sort */}
      <div className={styles.toolbar}>
        <p className={`label-sm ${styles.count}`}>
          {collection.items.length} {collection.items.length === 1 ? 'item' : 'items'}
        </p>
        <div className={styles.toolbarActions}>
          <SortControl
            options={COLLECTION_SORT_OPTIONS}
            field={sort.field}
            order={sort.order}
            onChange={setSort}
            className={styles.sortControl}
            selectClassName={styles.sortSelect}
            labelClassName={styles.sortLabel}
          />
          <button className="btn btn-primary" onClick={() => setShowAddMedia(true)}>
            <RiAddLine size={16} />
            Add Media
          </button>
        </div>
      </div>

      {/* Items grid */}
      {collection.items.length === 0 ? (
        <div className={styles.empty}>
          <p className="body-md" style={{ color: 'var(--color-on-surface-variant)' }}>
            No items yet. Add movies, TV shows, or audiobooks to this collection.
          </p>
        </div>
      ) : (
        <div className={styles.grid}>
          {sortedItems.map(item => (
            <RemovableCard
              key={item.id}
              item={item}
              collectionId={collection.id}
              onRemoved={handleItemRemoved}
            />
          ))}
        </div>
      )}

      {/* Edit modal */}
      {showEdit && (
        <div className={styles.modalBackdrop} onClick={() => setShowEdit(false)}>
          <div className={`glass ${styles.modal}`} onClick={e => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2 className="headline-sm" style={{ margin: 0 }}>Edit Collection</h2>
              <button className={styles.modalClose} onClick={() => setShowEdit(false)}><RiCloseLine size={20} /></button>
            </div>
            <form onSubmit={handleSaveEdit} className={styles.modalForm}>
              <div className={styles.field}>
                <label className="label-sm" htmlFor="edit-name">Name</label>
                <input id="edit-name" className="input" value={editName} onChange={e => setEditName(e.target.value)} required autoFocus />
              </div>
              <div className={styles.field}>
                <label className="label-sm" htmlFor="edit-desc">Description</label>
                <input id="edit-desc" className="input" value={editDesc} onChange={e => setEditDesc(e.target.value)} placeholder="Optional" />
              </div>
              <div className={styles.modalActions}>
                <button type="button" className="btn btn-outline" onClick={() => setShowEdit(false)}>Cancel</button>
                <button type="submit" className="btn btn-primary" disabled={saving || !editName.trim()}>
                  {saving ? 'Saving…' : 'Save'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add media modal */}
      {showAddMedia && (
        <AddMediaModal
          collectionId={collection.id}
          existingItems={collection.items}
          onItemAdded={handleItemAdded}
          onClose={() => setShowAddMedia(false)}
        />
      )}
    </div>
  )
}

// ── Removable card wrapper ────────────────────────────────

function RemovableCard({
  item, collectionId, onRemoved,
}: {
  item: CollectionItem
  collectionId: string
  onRemoved: (id: string) => void
}) {
  const [removing, setRemoving] = useState(false)
  const navigate = useNavigate()

  async function handleRemove(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    setRemoving(true)
    try {
      await api.removeCollectionItem(collectionId, item.id)
      onRemoved(item.id)
    } catch {
      setRemoving(false)
    }
  }

  // Detail-route prefix differs per media type. Audiobooks land on
  // /audiobook/:id where the user can pick a chapter (no direct "watch"
  // shortcut, since starting from chapter 1 isn't always what they want).
  const to = item.media_type === 'movie'
    ? `/movie/${item.media_id}`
    : item.media_type === 'audiobook'
      ? `/audiobook/${item.media_id}`
      : `/show/${item.media_id}`
  const onPlay = item.media_type === 'movie'
    ? () => navigate(`/movie/${item.media_id}/watch`)
    : undefined
  const fallbackIcon = item.media_type === 'movie'
    ? <RiFilmLine />
    : item.media_type === 'audiobook'
      ? <RiHeadphoneLine />
      : <RiTv2Line />

  return (
    <div className={styles.removableCard}>
      <MediaCard
        title={item.title}
        subtitle={item.year ? String(item.year) : undefined}
        imageSrc={item.poster_path || undefined}
        fallbackIcon={fallbackIcon}
        to={to}
        onPlay={onPlay}
      />
      <button
        className={styles.removeBtn}
        onClick={handleRemove}
        disabled={removing}
        aria-label={`Remove ${item.title}`}
      >
        <RiCloseLine size={14} />
      </button>
    </div>
  )
}

// ── Add media modal ───────────────────────────────────────

function AddMediaModal({
  collectionId, existingItems, onItemAdded, onClose,
}: {
  collectionId: string
  existingItems: CollectionItem[]
  onItemAdded: (item: CollectionItem) => void
  onClose: () => void
}) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [searching, setSearching] = useState(false)
  const [addingId, setAddingId] = useState<string | null>(null)
  const [added, setAdded] = useState<Set<string>>(() =>
    new Set(existingItems.map(i => `${i.media_type}:${i.media_id}`))
  )
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    const q = query.trim()
    if (!q) { setResults([]); return }
    setSearching(true)
    const timer = setTimeout(() => {
      api.search({ q })
        .then(res => {
          const items: SearchResultItem[] = []
          for (const lib of res.libraries) {
            for (const item of lib.items) {
              if (item.media_type === 'movie' || item.media_type === 'tvshow' || item.media_type === 'audiobook') {
                items.push(item)
              }
            }
          }
          setResults(items)
        })
        .finally(() => setSearching(false))
    }, 300)
    return () => clearTimeout(timer)
  }, [query])

  async function handleAdd(item: SearchResultItem) {
    const key = `${item.media_type}:${item.id}`
    if (added.has(key) || addingId) return
    setAddingId(item.id)
    try {
      const newItem = await api.addCollectionItem(collectionId, item.media_type, item.id)
      setAdded(prev => new Set([...prev, key]))
      onItemAdded(newItem)
    } catch {
      // already added or error — ignore
    } finally {
      setAddingId(null)
    }
  }

  return (
    <div className={styles.modalBackdrop} onClick={onClose}>
      <div className={`glass ${styles.addModal}`} onClick={e => e.stopPropagation()}>
        <div className={styles.modalHeader}>
          <h2 className="headline-sm" style={{ margin: 0 }}>Add Media</h2>
          <button className={styles.modalClose} onClick={onClose}><RiCloseLine size={20} /></button>
        </div>

        <input
          ref={inputRef}
          className={`input ${styles.searchInput}`}
          type="search"
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search movies, TV shows, and audiobooks…"
        />

        <div className={styles.searchResults}>
          {searching && (
            <p className={`label-sm ${styles.searchHint}`}>Searching…</p>
          )}
          {!searching && query.trim() && results.length === 0 && (
            <p className={`label-sm ${styles.searchHint}`}>No results</p>
          )}
          {!searching && !query.trim() && (
            <p className={`label-sm ${styles.searchHint}`}>Type to search for movies, TV shows, and audiobooks</p>
          )}
          {results.map(item => {
            const key = `${item.media_type}:${item.id}`
            const isAdded = added.has(key)
            return (
              <button
                key={item.id}
                className={`${styles.resultRow} ${isAdded ? styles.resultRowAdded : ''}`}
                onClick={() => handleAdd(item)}
                disabled={isAdded || addingId === item.id}
              >
                <div className={`card card-portrait ${styles.resultPoster}`}>
                  {item.poster_path ? (
                    <img src={imageUrl(item.poster_path)} alt={item.title} loading="lazy" />
                  ) : (
                    <div className={styles.resultPosterFallback}>
                      {item.media_type === 'movie'
                        ? <RiFilmLine size={18} />
                        : item.media_type === 'audiobook'
                          ? <RiHeadphoneLine size={18} />
                          : <RiTv2Line size={18} />}
                    </div>
                  )}
                </div>
                <div className={styles.resultMeta}>
                  <span className={`label-md ${styles.resultTitle}`}>{item.title}</span>
                  <span className={`label-sm ${styles.resultSub}`}>
                    {item.media_type === 'movie'
                      ? 'Movie'
                      : item.media_type === 'audiobook'
                        ? 'Audiobook'
                        : 'TV Show'}
                    {item.year > 0 ? ` · ${item.year}` : ''}
                  </span>
                </div>
                <div className={styles.resultAction}>
                  {isAdded ? (
                    <RiCheckLine size={18} className={styles.resultAdded} />
                  ) : addingId === item.id ? (
                    <span className={styles.resultSpinner} />
                  ) : (
                    <RiAddLine size={18} />
                  )}
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ── Loading skeleton ──────────────────────────────────────

function LoadingSkeleton() {
  return (
    <div className={`container ${styles.page}`}>
      <div className={styles.skeletonHeader} />
      <div className={styles.grid}>
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className={`card card-portrait ${styles.skeleton}`} />
        ))}
      </div>
    </div>
  )
}

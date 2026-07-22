import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RiAddLine, RiCloseLine } from 'react-icons/ri'
import { api } from '../api'
import type { Collection } from '../api'
import { CollectionPreview } from '../components/CollectionPreview'
import styles from './CollectionsPage.module.css'

export function CollectionsPage() {
  const navigate = useNavigate()
  const [collections, setCollections] = useState<Collection[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [createName, setCreateName] = useState('')
  const [createDesc, setCreateDesc] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')

  useEffect(() => {
    api.listCollections()
      .then(setCollections)
      .finally(() => setIsLoading(false))
  }, [])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!createName.trim()) return
    setCreating(true)
    setCreateError('')
    try {
      const col = await api.createCollection({ name: createName.trim(), description: createDesc.trim() })
      navigate(`/collections/${col.id}`)
    } catch {
      setCreateError('Failed to create collection')
      setCreating(false)
    }
  }

  function openCreate() {
    setCreateName('')
    setCreateDesc('')
    setCreateError('')
    setShowCreate(true)
  }

  if (isLoading) return <LoadingSkeleton />

  return (
    <div className={`container ${styles.page}`}>
      <h1 className={`headline-lg ${styles.heading}`}>Collections</h1>

      <div className={styles.grid}>
        {/* Create new card */}
        <button className={styles.newCard} onClick={openCreate} aria-label="New collection">
          <div className={styles.newCardIcon}>
            <RiAddLine size={28} />
          </div>
          <span className={`label-md ${styles.newCardLabel}`}>New Collection</span>
        </button>

        {collections.map(col => (
          <CollectionCard key={col.id} collection={col} onClick={() => navigate(`/collections/${col.id}`)} />
        ))}
      </div>

      {/* Create modal */}
      {showCreate && (
        <div className={styles.modalBackdrop} onClick={() => setShowCreate(false)}>
          <div className={`glass ${styles.modal}`} onClick={e => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2 className="headline-sm" style={{ margin: 0 }}>New Collection</h2>
              <button className={styles.modalClose} onClick={() => setShowCreate(false)} aria-label="Close">
                <RiCloseLine size={20} />
              </button>
            </div>

            <form onSubmit={handleCreate} className={styles.modalForm}>
              <div className={styles.field}>
                <label className="label-sm" htmlFor="col-name">Name</label>
                <input
                  id="col-name"
                  className="input"
                  type="text"
                  value={createName}
                  onChange={e => setCreateName(e.target.value)}
                  placeholder="e.g. Marvel Cinematic Universe"
                  autoFocus
                  required
                />
              </div>
              <div className={styles.field}>
                <label className="label-sm" htmlFor="col-desc">Description (optional)</label>
                <input
                  id="col-desc"
                  className="input"
                  type="text"
                  value={createDesc}
                  onChange={e => setCreateDesc(e.target.value)}
                  placeholder="What's this collection about?"
                />
              </div>
              {createError && (
                <p className="body-sm" style={{ color: 'var(--color-error)', margin: 0 }}>{createError}</p>
              )}
              <div className={styles.modalActions}>
                <button type="button" className="btn btn-outline" onClick={() => setShowCreate(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={creating || !createName.trim()}>
                  {creating ? 'Creating…' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

function CollectionCard({ collection, onClick }: { collection: Collection; onClick: () => void }) {
  return (
    <button className={styles.card} onClick={onClick}>
      <CollectionPreview covers={collection.covers ?? []} name={collection.name} emptyIconSize={36} />
      <div className={styles.cardMeta}>
        <p className={`label-md ${styles.cardName}`}>{collection.name}</p>
        {collection.description && (
          <p className={`label-sm ${styles.cardDesc}`}>{collection.description}</p>
        )}
      </div>
    </button>
  )
}

function LoadingSkeleton() {
  return (
    <div className={`container ${styles.page}`}>
      <div className={`${styles.skeletonHeading}`} />
      <div className={styles.grid}>
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className={styles.skeletonCard} />
        ))}
      </div>
    </div>
  )
}

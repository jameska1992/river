import { useEffect, useRef, useState } from 'react'
import { RiMoreLine, RiRefreshLine, RiEditLine, RiSearchLine, RiInformationLine, RiDeleteBin6Line } from 'react-icons/ri'
import styles from './AdminMediaMenu.module.css'

interface Props {
  onRefresh?: () => Promise<void>
  onEdit: () => void
  onIdentify?: () => void
  // onShowDetails opens a read-only modal that dumps every field on the
  // record — source paths, IDs, timestamps, etc. Aimed at admin debug
  // workflows where you want to see exactly what's in the database.
  onShowDetails?: () => void
  // onDelete opens a confirmation modal that lets the admin pick between
  // remove-from-db and remove-and-delete-files. Left optional so existing
  // call sites that haven't wired it up yet still compile.
  onDelete?: () => void
  // identifyLabel lets callers customize what the Identify menu entry says
  // — useful since this menu is reused for movies, series, etc. Defaults
  // to the generic "Identify" so it reads sensibly in any context.
  identifyLabel?: string
}

export function AdminMediaMenu({ onRefresh, onEdit, onIdentify, onShowDetails, onDelete, identifyLabel }: Props) {
  const [open, setOpen] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle')
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const handleRefresh = async () => {
    setOpen(false)
    setRefreshing(true)
    setStatus('idle')
    try {
      await onRefresh?.()
      setStatus('success')
      setTimeout(() => setStatus('idle'), 3000)
    } catch {
      setStatus('error')
      setTimeout(() => setStatus('idle'), 3000)
    } finally {
      setRefreshing(false)
    }
  }

  const handleEdit = () => {
    setOpen(false)
    onEdit()
  }

  const handleIdentify = () => {
    setOpen(false)
    onIdentify?.()
  }

  const handleDetails = () => {
    setOpen(false)
    onShowDetails?.()
  }

  const handleDelete = () => {
    setOpen(false)
    onDelete?.()
  }

  return (
    <div className={styles.root} ref={menuRef}>
      <button
        className={`btn btn-icon ${styles.trigger} ${refreshing ? styles.spinning : ''} ${status === 'success' ? styles.success : ''} ${status === 'error' ? styles.error : ''}`}
        onClick={() => setOpen(o => !o)}
        disabled={refreshing}
        aria-label="Admin options"
        title={status === 'success' ? 'Refresh queued' : status === 'error' ? 'Refresh failed' : undefined}
      >
        <RiMoreLine size={20} />
      </button>

      {open && (
        <div className={styles.menu} role="menu">
          <button className={styles.item} onClick={handleRefresh} role="menuitem">
            <RiRefreshLine size={16} />
            <span>Refresh metadata</span>
          </button>
          <button className={styles.item} onClick={handleEdit} role="menuitem">
            <RiEditLine size={16} />
            <span>Edit metadata</span>
          </button>
          {onIdentify && (
            <button className={styles.item} onClick={handleIdentify} role="menuitem">
              <RiSearchLine size={16} />
              <span>{identifyLabel ?? 'Identify'}</span>
            </button>
          )}
          {onShowDetails && (
            <button className={styles.item} onClick={handleDetails} role="menuitem">
              <RiInformationLine size={16} />
              <span>Show details</span>
            </button>
          )}
          {onDelete && (
            <button className={`${styles.item} ${styles.itemDanger}`} onClick={handleDelete} role="menuitem">
              <RiDeleteBin6Line size={16} />
              <span>Delete…</span>
            </button>
          )}
        </div>
      )}
    </div>
  )
}

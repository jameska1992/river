import { useEffect, useRef, useState } from 'react'
import { RiCloseLine, RiDeleteBin6Line, RiAlertLine } from 'react-icons/ri'
import styles from './DeleteMediaModal.module.css'

interface Props {
  // mediaLabel is what gets read back to the user ("Movie", "TV show",
  // "Album", etc.) so the same modal can serve every entity type.
  mediaLabel: string
  // title shows what's being deleted ("Avatar (2024)") — pure UI text.
  title: string
  // onConfirm receives whether the user picked the destructive "also
  // delete files on disk" option. Resolves once the delete completes;
  // the modal owns the in-flight spinner state.
  onConfirm: (deleteFiles: boolean) => Promise<void>
  onClose: () => void
}

export function DeleteMediaModal({ mediaLabel, title, onConfirm, onClose }: Props) {
  const [busy, setBusy] = useState<'remove' | 'remove-files' | null>(null)
  const [error, setError] = useState<string | null>(null)
  const overlayRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      // Don't dismiss while a delete is in flight — the user can't see
      // the result yet and an accidental Escape would orphan the call.
      if (e.key === 'Escape' && !busy) onClose()
    }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose, busy])

  const handle = async (deleteFiles: boolean) => {
    setBusy(deleteFiles ? 'remove-files' : 'remove')
    setError(null)
    try {
      await onConfirm(deleteFiles)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setBusy(null)
    }
  }

  return (
    <div
      ref={overlayRef}
      className={styles.overlay}
      onMouseDown={e => { if (e.target === overlayRef.current && !busy) onClose() }}
    >
      <div className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="delete-modal-title">
        <div className={styles.header}>
          <h2 id="delete-modal-title" className={`headline-sm ${styles.headerTitle}`}>
            Delete {mediaLabel}
          </h2>
          <button className="btn btn-icon" onClick={onClose} disabled={!!busy} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <div className={styles.body}>
          <p className={styles.subtitle}>{title}</p>

          <p className={styles.lead}>
            What would you like to do?
          </p>

          <button
            className={styles.option}
            onClick={() => handle(false)}
            disabled={!!busy}
          >
            <RiDeleteBin6Line size={20} className={styles.optionIcon} />
            <span className={styles.optionBody}>
              <span className={styles.optionTitle}>Remove {mediaLabel}</span>
              <span className={styles.optionDesc}>
                Removes the database entries and clears the scanner state hash so
                the content can be re-discovered on the next scan. Files on disk
                are left untouched.
              </span>
            </span>
            {busy === 'remove' && <span className={styles.spinner} aria-hidden />}
          </button>

          <button
            className={`${styles.option} ${styles.optionDanger}`}
            onClick={() => handle(true)}
            disabled={!!busy}
          >
            <RiAlertLine size={20} className={styles.optionIcon} />
            <span className={styles.optionBody}>
              <span className={styles.optionTitle}>Remove and delete media</span>
              <span className={styles.optionDesc}>
                Same as Remove, then also deletes the source files from disk.
                This cannot be undone.
              </span>
            </span>
            {busy === 'remove-files' && <span className={styles.spinner} aria-hidden />}
          </button>

          {error && <p className={styles.error}>{error}</p>}
        </div>

        <div className={styles.footer}>
          <button className="btn btn-text" onClick={onClose} disabled={!!busy}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

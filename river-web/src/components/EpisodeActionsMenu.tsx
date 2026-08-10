import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import { RiMoreLine, RiGroupLine, RiDownloadLine, RiHdLine, RiEditLine, RiDeleteBin6Line } from 'react-icons/ri'
import styles from './EpisodeActionsMenu.module.css'

interface Props {
  onWatchParty: () => void
  // downloadUrl is set only when the episode has a transcoded file to grab.
  downloadUrl?: string
  // originalPath is the router target for the untranscoded source variant,
  // set only when a distinct source exists.
  originalPath?: string
  isAdmin: boolean
  onEdit: () => void
  onDelete: () => void
}

type Coords = { top?: number; bottom?: number; right: number }

// EpisodeActionsMenu collapses every per-episode action except Play into a
// single "⋯" dropdown, keeping the episode row uncluttered. Items are a mix of
// buttons (watch party, edit, delete) and navigations (download link, original
// source), all sharing one item style so the menu reads consistently.
//
// The menu is rendered in a portal with fixed positioning: the episode list
// clips overflow for its expand/collapse animation, so an in-flow dropdown
// would be cut off on the last rows. Fixed coords are anchored to the trigger
// and flipped above it when there isn't room below.
export function EpisodeActionsMenu({ onWatchParty, downloadUrl, originalPath, isAdmin, onEdit, onDelete }: Props) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState<Coords | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  const place = () => {
    const el = triggerRef.current
    if (!el) return
    const r = el.getBoundingClientRect()
    const right = window.innerWidth - r.right
    // Rough height estimate (item ≈ 40px + a little chrome) is enough to decide
    // whether to flip upward; the exact height doesn't matter for the choice.
    const itemCount = 1 + (downloadUrl ? 1 : 0) + (originalPath ? 1 : 0) + (isAdmin ? 2 : 0)
    const estHeight = itemCount * 40 + 8
    const spaceBelow = window.innerHeight - r.bottom
    if (spaceBelow < estHeight + 12 && r.top > estHeight + 12) {
      setCoords({ bottom: window.innerHeight - r.top + 6, right })
    } else {
      setCoords({ top: r.bottom + 6, right })
    }
  }

  const toggle = () => {
    if (!open) place()
    setOpen(o => !o)
  }

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (triggerRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    // Fixed coords go stale on scroll/resize — closing is the simplest correct
    // behaviour (matches how most menus behave when the page moves under them).
    const onMove = () => setOpen(false)
    document.addEventListener('mousedown', onDown)
    window.addEventListener('scroll', onMove, true)
    window.addEventListener('resize', onMove)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('scroll', onMove, true)
      window.removeEventListener('resize', onMove)
    }
  }, [open])

  const close = () => setOpen(false)
  // run wraps a button action so the menu closes before it fires.
  const run = (fn: () => void) => () => { setOpen(false); fn() }

  return (
    <div className={styles.root}>
      <button
        ref={triggerRef}
        className={`btn btn-icon ${styles.trigger}`}
        onClick={toggle}
        aria-label="Episode options"
        aria-haspopup="menu"
        aria-expanded={open}
        title="More"
      >
        <RiMoreLine size={18} />
      </button>

      {open && coords && createPortal(
        <div
          ref={menuRef}
          className={styles.menu}
          role="menu"
          style={{ top: coords.top, bottom: coords.bottom, right: coords.right }}
        >
          <button className={styles.item} onClick={run(onWatchParty)} role="menuitem">
            <RiGroupLine size={16} />
            <span>Start watch party</span>
          </button>
          {downloadUrl && (
            <a className={styles.item} href={downloadUrl} onClick={close} role="menuitem">
              <RiDownloadLine size={16} />
              <span>Download</span>
            </a>
          )}
          {originalPath && (
            <Link
              className={styles.item}
              to={originalPath}
              onClick={close}
              role="menuitem"
              title="Original (untranscoded) — may not play in every browser"
            >
              <RiHdLine size={16} />
              <span>Original (untranscoded)</span>
            </Link>
          )}
          {isAdmin && (
            <>
              <button className={styles.item} onClick={run(onEdit)} role="menuitem">
                <RiEditLine size={16} />
                <span>Edit metadata</span>
              </button>
              <button className={`${styles.item} ${styles.itemDanger}`} onClick={run(onDelete)} role="menuitem">
                <RiDeleteBin6Line size={16} />
                <span>Delete…</span>
              </button>
            </>
          )}
        </div>,
        document.body,
      )}
    </div>
  )
}

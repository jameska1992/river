import { Link } from 'react-router-dom'
import { RiMoreLine, RiGroupLine, RiCheckLine, RiCloseCircleLine, RiDownloadLine, RiHdLine, RiEditLine, RiRestartLine, RiRewindStartLine, RiDeleteBin6Line, RiClosedCaptioningLine } from 'react-icons/ri'
import { DropdownMenu } from './DropdownMenu'
import dropdownStyles from './DropdownMenu.module.css'
import styles from './EpisodeActionsMenu.module.css'

interface Props {
  // startFromPath is the router target that forces playback from the
  // beginning (…/watch?startFrom=0), bypassing saved-progress resume.
  startFromPath: string
  onWatchParty: () => void
  // watched reflects the current user's completion state for this episode;
  // it flips the toggle item's label/icon.
  watched: boolean
  onToggleWatched: () => void
  // downloadUrl is set only when the episode has a transcoded file to grab.
  downloadUrl?: string
  // originalPath is the router target for the untranscoded source variant,
  // set only when a distinct source exists.
  originalPath?: string
  isAdmin: boolean
  onEdit: () => void
  onReTranscode: () => void
  onSearchSubtitles: () => void
  onDelete: () => void
}

// EpisodeActionsMenu collapses every per-episode action except Play into a
// single "⋯" dropdown, keeping the episode row uncluttered. Items are a mix of
// buttons (watch party, edit, delete) and navigations (download link, original
// source), all sharing one item style so the menu reads consistently. The
// portal/positioning/close mechanics live in DropdownMenu.
export function EpisodeActionsMenu({ startFromPath, onWatchParty, watched, onToggleWatched, downloadUrl, originalPath, isAdmin, onEdit, onReTranscode, onSearchSubtitles, onDelete }: Props) {
  return (
    <DropdownMenu
      menuLabel="Episode options"
      trigger={({ ref, toggle, open }) => (
        <button
          ref={ref}
          className={`btn btn-icon ${styles.trigger}`}
          onClick={toggle}
          aria-label="Episode options"
          aria-haspopup="menu"
          aria-expanded={open}
          title="More"
        >
          <RiMoreLine size={18} />
        </button>
      )}
    >
      {close => {
        // run wraps a button action so the menu closes before it fires.
        const run = (fn: () => void) => () => { close(); fn() }
        return (
          <>
            <Link className={dropdownStyles.item} to={startFromPath} onClick={close} role="menuitem">
              <RiRewindStartLine size={16} />
              <span>Play from beginning</span>
            </Link>
            <button className={dropdownStyles.item} onClick={run(onWatchParty)} role="menuitem">
              <RiGroupLine size={16} />
              <span>Start watch party</span>
            </button>
            <button className={dropdownStyles.item} onClick={run(onToggleWatched)} role="menuitem">
              {watched ? <RiCloseCircleLine size={16} /> : <RiCheckLine size={16} />}
              <span>{watched ? 'Mark unwatched' : 'Mark watched'}</span>
            </button>
            {downloadUrl && (
              <a className={dropdownStyles.item} href={downloadUrl} onClick={close} role="menuitem">
                <RiDownloadLine size={16} />
                <span>Download</span>
              </a>
            )}
            {originalPath && (
              <Link
                className={dropdownStyles.item}
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
                <button className={dropdownStyles.item} onClick={run(onEdit)} role="menuitem">
                  <RiEditLine size={16} />
                  <span>Edit metadata</span>
                </button>
                <button className={dropdownStyles.item} onClick={run(onReTranscode)} role="menuitem">
                  <RiRestartLine size={16} />
                  <span>Re-transcode</span>
                </button>
                <button className={dropdownStyles.item} onClick={run(onSearchSubtitles)} role="menuitem">
                  <RiClosedCaptioningLine size={16} />
                  <span>Search subtitles</span>
                </button>
                <button className={`${dropdownStyles.item} ${dropdownStyles.itemDanger}`} onClick={run(onDelete)} role="menuitem">
                  <RiDeleteBin6Line size={16} />
                  <span>Delete…</span>
                </button>
              </>
            )}
          </>
        )
      }}
    </DropdownMenu>
  )
}

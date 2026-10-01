import { RiDownloadLine, RiHdLine } from 'react-icons/ri'
import { DropdownMenu } from './DropdownMenu'
import dropdownStyles from './DropdownMenu.module.css'

interface Props {
  // transcodedUrl is set when a transcoded file is available to download.
  transcodedUrl?: string
  // originalUrl is set when a distinct, untranscoded source file exists.
  originalUrl?: string
}

// MovieDownloadButton renders the movie download control. When both a
// transcoded file and a distinct original exist it offers a dropdown with each;
// with only one available it collapses to a plain icon button; with neither it
// renders nothing. Mirrors the EpisodeActionsMenu download items, but both
// entries are true downloads (the backend serves them Content-Disposition:
// attachment), including the original via `?variant=source`.
export function MovieDownloadButton({ transcodedUrl, originalUrl }: Props) {
  if (!transcodedUrl && !originalUrl) return null

  // Exactly one variant → a plain icon button, no dropdown toggle.
  if (!transcodedUrl || !originalUrl) {
    const url = transcodedUrl ?? originalUrl!
    const isOriginal = !transcodedUrl
    return (
      <a
        href={url}
        className="btn btn-icon"
        title={isOriginal ? 'Download original (untranscoded)' : 'Download'}
        aria-label={isOriginal ? 'Download original movie file' : 'Download movie'}
      >
        <RiDownloadLine size={18} />
      </a>
    )
  }

  return (
    <DropdownMenu
      menuLabel="Download options"
      trigger={({ ref, toggle, open }) => (
        <button
          ref={ref}
          className="btn btn-icon"
          onClick={toggle}
          aria-label="Download options"
          aria-haspopup="menu"
          aria-expanded={open}
          title="Download"
        >
          <RiDownloadLine size={18} />
        </button>
      )}
    >
      {close => (
        <>
          <a className={dropdownStyles.item} href={transcodedUrl} onClick={close} role="menuitem">
            <RiDownloadLine size={16} />
            <span>Download</span>
          </a>
          <a
            className={dropdownStyles.item}
            href={originalUrl}
            onClick={close}
            role="menuitem"
            title="The original, untranscoded file (full quality, larger)"
          >
            <RiHdLine size={16} />
            <span>Download original</span>
          </a>
        </>
      )}
    </DropdownMenu>
  )
}

import { useNavigate } from 'react-router-dom'
import { RiGroupLine, RiLinksLine, RiStopCircleLine, RiLogoutBoxLine } from 'react-icons/ri'
import { api } from '../api'
import type { WatchPartyMember } from '../api'
import styles from './WatchPartyOverlay.module.css'

interface Props {
  roomId: string
  members: WatchPartyMember[]
  isHost: boolean
  backPath: string
  onClick?: (e: React.MouseEvent) => void
  onDoubleClick?: (e: React.MouseEvent) => void
}

export function WatchPartyOverlay({ roomId, members, isHost, backPath, onClick, onDoubleClick }: Props) {
  const navigate = useNavigate()

  const copyLink = () => {
    navigator.clipboard.writeText(window.location.href).catch(() => {})
  }

  const handleLeave = async () => {
    if (isHost) {
      try { await api.deleteWatchParty(roomId) } catch { /* ignore */ }
    }
    navigate(backPath, { replace: true })
  }

  return (
    <div className={styles.overlay} onClick={onClick} onDoubleClick={onDoubleClick}>
      <div className={styles.header}>
        <RiGroupLine size={14} />
        <span className="label-sm">Watch Party</span>
        <span className={`label-sm ${styles.count}`}>{members.length}</span>
      </div>

      {members.length > 0 && (
        <ul className={styles.memberList}>
          {members.map(m => (
            <li key={m.user_id} className={`label-sm ${styles.member}`}>{m.username}</li>
          ))}
        </ul>
      )}

      <div className={styles.actions}>
        <button className={`btn btn-icon ${styles.actionBtn}`} onClick={copyLink} title="Copy invite link">
          <RiLinksLine size={15} />
        </button>
        <button className={`btn btn-icon ${styles.actionBtn} ${styles.leaveBtn}`} onClick={handleLeave} title={isHost ? 'End party' : 'Leave party'}>
          {isHost ? <RiStopCircleLine size={15} /> : <RiLogoutBoxLine size={15} />}
        </button>
      </div>
    </div>
  )
}

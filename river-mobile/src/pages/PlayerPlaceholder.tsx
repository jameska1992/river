import { useNavigate } from 'react-router-dom'
import { RiArrowLeftLine } from 'react-icons/ri'
import { screen, heading } from './styles'

// Temporary stand-in for the players. The touch video player (#143) and audio
// player (#144) replace this; it exists so Play affordances on detail pages
// route somewhere real in the browse-only scaffold.
export default function PlayerPlaceholder() {
  const navigate = useNavigate()
  return (
    <div style={screen}>
      <button className="btn" onClick={() => navigate(-1)} style={{ gap: '0.5rem', marginBottom: '1rem' }}>
        <RiArrowLeftLine /> Back
      </button>
      <h1 style={heading}>Player coming soon</h1>
      <p style={{ color: 'var(--text-muted)' }}>
        Playback lands in the next milestones (video #143, audio #144).
      </p>
    </div>
  )
}

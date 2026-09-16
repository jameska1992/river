import { useAuth } from '../context/authContext'
import { screen, heading } from './styles'

export default function HomePage() {
  const { user } = useAuth()
  return (
    <div style={screen}>
      <h1 style={heading}>River</h1>
      <p style={{ color: 'var(--text-muted)' }}>
        Welcome{user ? `, ${user.username}` : ''}. The mobile app scaffold is up —
        Home rails (continue watching, recently added, next up) land next.
      </p>
    </div>
  )
}

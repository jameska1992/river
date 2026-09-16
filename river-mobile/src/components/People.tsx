import { Row, RailItem } from './collections'
import { imageUrl } from '../util/imageUrl'
import { initials } from '../util/format'
import type { PersonEntry } from '../util/credits'

// A horizontal rail of people (cast or crew). Tiles are non-linking — there's
// no person page on mobile — so they're plain avatars with a name + role.
export function PeopleRow({ title, people }: { title: string; people: PersonEntry[] }) {
  if (people.length === 0) return null
  return (
    <Row title={title}>
      {people.map(p => {
        const src = imageUrl(p.imagePath)
        return (
          <RailItem key={p.key} width="5.5rem">
            <div style={styles.tile}>
              <span style={styles.avatar}>
                {src
                  ? <img src={src} alt={p.name} loading="lazy" style={styles.avatarImg} />
                  : initials(p.name)}
              </span>
              <span style={styles.name}>{p.name}</span>
              {p.role && <span style={styles.role}>{p.role}</span>}
            </div>
          </RailItem>
        )
      })}
    </Row>
  )
}

const styles: Record<string, React.CSSProperties> = {
  tile: { display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' },
  avatar: {
    width: '5.5rem', height: '5.5rem', borderRadius: '50%',
    display: 'grid', placeItems: 'center', overflow: 'hidden',
    background: 'var(--bg-elev-3)', color: 'var(--text-muted)', fontWeight: 700,
  },
  avatarImg: { width: '100%', height: '100%', objectFit: 'cover' },
  name: {
    marginTop: '0.4rem', fontSize: '0.75rem', fontWeight: 600, width: '100%',
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  role: {
    fontSize: '0.7rem', color: 'var(--text-muted)', width: '100%',
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
}

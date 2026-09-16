import { useParams } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../hooks/useAsync'
import { Grid } from '../components/collections'
import { PosterCard } from '../components/PosterCard'
import { Loading, ErrorState, EmptyState } from '../components/States'
import { detailPath } from '../util/format'
import { heading, screen } from './styles'

export default function CollectionDetailPage() {
  const { id = '' } = useParams()
  const { data, loading, error, reload } = useAsync(() => api.getCollection(id), [id])

  if (loading) return <Loading />
  if (error || !data) return <ErrorState message={error ?? 'Not found'} onRetry={reload} />

  return (
    <div>
      <h1 style={{ ...heading, ...screen, paddingBottom: data.description ? '0.25rem' : '0.75rem' }}>{data.name}</h1>
      {data.description && (
        <p style={{ padding: '0 1.25rem 1rem', margin: 0, color: 'var(--text-muted)' }}>{data.description}</p>
      )}

      {data.items.length === 0 ? <EmptyState message="This collection is empty." /> : (
        <Grid>
          {data.items.map(item => (
            <PosterCard
              key={item.id}
              to={detailPath(item.media_type, item.media_id)}
              title={item.title}
              subtitle={item.year && item.year > 0 ? String(item.year) : undefined}
              image={item.poster_path}
              kind={item.media_type}
            />
          ))}
        </Grid>
      )}
    </div>
  )
}
